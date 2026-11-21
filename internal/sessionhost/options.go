package sessionhost

import (
	"crypto/rand"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"studio/internal/acp"
	"studio/internal/agents"
	"time"
)

//go:embed options.go
var OptionsSource []byte

// OptionsHost retains provider transport and settings persistence in the host.
// Save runs after the conversation has released its private mutex.
type OptionsHost struct {
	Conversation func(string) *Conversation
	Connection   func(*Conversation) Connection
	Call         func(Connection, string, any, any) error
	Save         func(*Conversation, []acp.ConfigOption, map[string]json.RawMessage) error
	Emit         func(string, any)
}
type Options struct {
	host OptionsHost
	// RestoreLimit replaces RestoreSettingsLimit when it is positive, which a
	// test uses to meet the limit quickly. It is set before the first open.
	RestoreLimit time.Duration
}

func NewOptions(host OptionsHost) *Options { return &Options{host: host} }
func (o *Options) Finish(s *Conversation, opts []acp.ConfigOption) error {
	if err := o.Update(s, opts, true); err != nil {
		return err
	}
	s.SetRestoring(false)
	return nil
}
func (o *Options) Update(s *Conversation, opts []acp.ConfigOption, finishRestore bool) error {
	before := agents.ModelName(s.Options())
	restoring, kept := s.UpdateOptions(opts)
	var saveErr error
	if !restoring || finishRestore {
		saveErr = o.host.Save(s, opts, kept)
	}
	EmitOptions(o.host.Emit, s, saveErr)
	// The chat's information has its model's name (Info.Model), which the
	// Chats list and the Org Chart show, so a change of model sends it again.
	if agents.ModelName(s.Options()) != before {
		EmitSession(o.host.Emit, s)
	}
	return saveErr
}

func (o *Options) SetOption(conn Connection, session string, option acp.ConfigOption, value json.RawMessage) ([]acp.ConfigOption, error) {
	p := acp.SetConfigOptionParams{SessionID: session, ConfigID: option.ID, Value: value}
	if option.Type == "boolean" {
		p.Type = "boolean"
	}
	var res acp.ConfigOptionsResult
	if err := o.host.Call(conn, "session/set_config_option", p, &res); err != nil {
		return nil, err
	}
	return res.ConfigOptions, nil
}

func (o *Options) Set(id, option string, value json.RawMessage) ([]acp.ConfigOption, error) {
	opts, _, err := o.SetRecorded(id, option, value, nil)
	return opts, err
}

// OptionChange is an observation of a temporary option write. Its revision is
// valid only in the same runtime and is invalidated by every explicit setter.
type OptionChange struct {
	SessionID string          `json:"sessionId"`
	OptionID  string          `json:"optionId"`
	From      json.RawMessage `json:"from"`
	To        json.RawMessage `json:"to"`
	Revision  string          `json:"revision"`
	Applied   bool            `json:"applied"`
}

var ErrOptionsChanged = errors.New("the chat's provider settings changed after this task's model choice")
var ErrOptionUnconfirmed = errors.New("the provider did not confirm the requested setting value")
var ErrOptionBusy = errors.New("the chat is not idle for a task configuration change")

// SetRecorded serializes ordinary setters and temporary changes. Capture runs
// before native dispatch and again after the returned options are observed.
// It must persist synchronously and must not reenter a configuration setter.
func (o *Options) SetRecorded(id, option string, value json.RawMessage, capture func(OptionChange) error) ([]acp.ConfigOption, OptionChange, error) {
	s := o.host.Conversation(id)
	if s == nil {
		return nil, OptionChange{}, errors.New("that session is not open")
	}
	s.configuration.Lock()
	defer s.configuration.Unlock()
	return o.setRecorded(s, option, value, "", capture)
}

func (o *Options) RestoreRecorded(change OptionChange, capture func(OptionChange) error) ([]acp.ConfigOption, OptionChange, error) {
	s := o.host.Conversation(change.SessionID)
	if s == nil {
		return nil, OptionChange{}, errors.New("that session is not open")
	}
	s.configuration.Lock()
	defer s.configuration.Unlock()
	s.mu.Lock()
	current := s.details.optionRevision
	var value json.RawMessage
	for _, opt := range s.details.options {
		if opt.ID == change.OptionID {
			value = opt.CurrentValue
			break
		}
	}
	s.mu.Unlock()
	if !change.Applied || change.Revision == "" || current != change.Revision || !SameOptionValue(value, change.To) {
		return s.Options(), OptionChange{}, ErrOptionsChanged
	}
	return o.setRecorded(s, change.OptionID, change.From, change.Revision, capture)
}

func (o *Options) setRecorded(s *Conversation, option string, value json.RawMessage, expected string, capture func(OptionChange) error) ([]acp.ConfigOption, OptionChange, error) {
	id := s.ID()
	conn := o.host.Connection(s)
	if conn == nil {
		return nil, OptionChange{}, errors.New("that session is not open")
	}

	loading, opt := s.OptionForChange(option)
	if loading {
		return nil, OptionChange{}, errors.New("wait for this chat's provider settings to finish restoring")
	}
	if opt == nil {
		return nil, OptionChange{}, errors.New("the agent offers no such setting now")
	}
	if !agents.Allowed(*opt, value) {
		return nil, OptionChange{}, fmt.Errorf("the agent does not offer that value for %s", opt.Name)
	}
	s.mu.Lock()
	if expected != "" && s.details.optionRevision != expected {
		s.mu.Unlock()
		return nil, OptionChange{}, ErrOptionsChanged
	}
	exclusive := capture != nil || expected != ""
	if exclusive && (!s.activity.ReadyLocked() || s.activity.ContinuationBusyLocked(false) || s.details.BackgroundCountLocked() != 0 || s.details.RunningSubagentsLocked() != 0) {
		s.mu.Unlock()
		return nil, OptionChange{}, ErrOptionBusy
	}
	found := false
	for _, current := range s.details.options {
		if current.ID == option {
			found = true
			copy := current
			copy.CurrentValue = append(json.RawMessage(nil), current.CurrentValue...)
			copy.Options = append(json.RawMessage(nil), current.Options...)
			opt = &copy
			break
		}
	}
	if !found || !agents.Allowed(*opt, value) {
		s.mu.Unlock()
		return nil, OptionChange{}, ErrOptionsChanged
	}
	if exclusive {
		s.details.configuring = true
	}
	s.details.optionRevision = rand.Text()
	change := OptionChange{SessionID: id, OptionID: option, From: append(json.RawMessage(nil), opt.CurrentValue...), To: append(json.RawMessage(nil), value...), Revision: s.details.optionRevision}
	s.mu.Unlock()
	if exclusive {
		defer func() {
			s.mu.Lock()
			s.details.configuring = false
			s.mu.Unlock()
			EmitSession(o.host.Emit, s)
		}()
		EmitSession(o.host.Emit, s)
	}
	if capture != nil {
		if err := capture(change); err != nil {
			return nil, change, err
		}
	}
	opts, err := o.SetOption(conn, id, *opt, value)
	if err != nil {
		return nil, change, err
	}
	// The person's own choice becomes the chat's saved value, in place of
	// one a substitution kept.
	s.ClearOverride(option)
	err = o.Update(s, opts, false)
	s.mu.Lock()
	change.Revision = s.details.optionRevision
	for _, opt := range s.details.options {
		if opt.ID == option {
			change.Applied = SameOptionValue(opt.CurrentValue, value)
			break
		}
	}
	s.mu.Unlock()
	if !change.Applied && exclusive {
		err = errors.Join(err, ErrOptionUnconfirmed)
	}
	if capture != nil {
		err = errors.Join(err, capture(change))
	}
	return opts, change, err
}
