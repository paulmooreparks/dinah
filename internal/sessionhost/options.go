package sessionhost

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"studio/internal/acp"
	"studio/internal/agents"
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
type Options struct{ host OptionsHost }

func NewOptions(host OptionsHost) *Options { return &Options{host: host} }
func (o *Options) Finish(s *Conversation, opts []acp.ConfigOption) error {
	if err := o.Update(s, opts, true); err != nil {
		return err
	}
	s.SetRestoring(false)
	return nil
}
func (o *Options) Update(s *Conversation, opts []acp.ConfigOption, finishRestore bool) error {
	restoring, kept := s.UpdateOptions(opts)
	var saveErr error
	if !restoring || finishRestore {
		saveErr = o.host.Save(s, opts, kept)
	}
	EmitOptions(o.host.Emit, s, saveErr)
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
	s := o.host.Conversation(id)
	conn := o.host.Connection(s)
	if s == nil || conn == nil {
		return nil, errors.New("that session is not open")
	}

	loading, opt := s.OptionForChange(option)
	if loading {
		return nil, errors.New("wait for this chat's provider settings to finish restoring")
	}
	if opt == nil {
		return nil, errors.New("the agent offers no such setting now")
	}
	if !agents.Allowed(*opt, value) {
		return nil, fmt.Errorf("the agent does not offer that value for %s", opt.Name)
	}
	opts, err := o.SetOption(conn, id, *opt, value)
	if err != nil {
		return nil, err
	}
	// The person's own choice becomes the chat's saved value, in place of
	// one a substitution kept.
	s.ClearOverride(option)
	err = o.Update(s, opts, false)
	return opts, err
}
