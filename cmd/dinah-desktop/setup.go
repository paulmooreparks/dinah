package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"

	"studio/internal/acp"
	"studio/internal/judge"
	"studio/internal/profile"
	"studio/internal/providers"
	"studio/internal/workspaces"
)

// The guided set-up.
//
// A new installation started with every setting at its default and nothing
// to say which mattered. Paul asked on 20 November 2026 for a set-up that
// the default agent guides: it opens by itself once, asks a few questions,
// shows everything it would set before anything is set, and puts it back on
// Undo (docs/mockups/guided-setup, which he approved in its third form).
//
// What the product can tell and do decides the set-up's words. Whether a
// person is signed in to an agent cannot be read, so a row says only whether
// the agent's program was found, and choosing one checks its connection
// (AgentCheck). What an agent asks before it acts is each provider's own
// setting with its own choices, so the question offers those. Undo cannot
// sign anybody out, and it turns judgements off again, since the answer to
// their question cannot be taken back.

// SetupChoice is one choice of a question: what is saved, and what is read.
type SetupChoice struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

// SetupAsking is a provider's setting for what it asks before it acts: its
// choices and the one that holds now.
type SetupAsking struct {
	Choices []SetupChoice `json:"choices"`
	Current string        `json:"current"`
}

// SetupAgent is one agent provider as the set-up's first step lists it.
type SetupAgent struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	// Found says its program was found, and Words say what is wrong when
	// it was not, in the words Settings uses.
	Found bool   `json:"found"`
	Words string `json:"words"`
	// SignIn says the window can sign the person in to it, and InstallPage
	// that there is a page to open that says how to install it.
	SignIn      bool `json:"signIn"`
	InstallPage bool `json:"installPage"`
	// Judges says its models can answer the small questions in the
	// background.
	Judges bool `json:"judges"`
}

// SetupUse is one answer to what the product is used for most.
type SetupUse struct {
	ID      string   `json:"id"`
	Label   string   `json:"label"`
	Windows []string `json:"windows"`
	Opens   string   `json:"opens"`
}

// SetupOffer is what the set-up's window needs to ask its questions.
type SetupOffer struct {
	// Shown says the window has opened by itself once already.
	Shown   bool         `json:"shown"`
	Default string       `json:"default"`
	Agents  []SetupAgent `json:"agents"`
	Uses    []SetupUse   `json:"uses"`
	// PersonName is the name the person chose, or empty, and AccountName
	// the one the computer's account has, which is used when none is
	// chosen.
	PersonName  string `json:"personName"`
	AccountName string `json:"accountName"`
	// Workspace is the folder of the workspace in front, or empty.
	Workspace string `json:"workspace"`
	// JudgeOn says the small questions in the background are answered
	// already, so the set-up does not ask about them. JudgeName is the
	// agent that would answer them when the chosen one cannot, or empty
	// when none can, and JudgeCalls the most calls a day.
	JudgeOn    bool   `json:"judgeOn"`
	JudgeName  string `json:"judgeName"`
	JudgeCalls int    `json:"judgeCalls"`
}

// setupUses are the set-up's own answers to what the product is used for
// most, with the windows each opens; a product's profile may give others.
var setupUses = []SetupUse{
	{ID: "code", Label: "Writing code with an agent", Windows: []string{"workspace", "files", "terminal"}, Opens: "Workspace, Files and Terminal"},
	{ID: "docs", Label: "Writing documents and diagrams", Windows: []string{"workspace", "files", "in-progress"}, Opens: "Workspace, Files and In Progress"},
	{ID: "team", Label: "Running several agents as a team", Windows: []string{"chat-browser", "org-chart", "needs-you", "decisions"}, Opens: "Chats, Org Chart, Notifications and Decisions"},
	{ID: "all", Label: "A bit of everything", Windows: []string{"workspace", "chat-browser", "needs-you"}, Opens: "Workspace, Chats and Notifications"},
}

// offeredSetupUses are the uses this product's set-up offers.
func offeredSetupUses() []SetupUse { return setupUsesOf(product) }

// setupUsesOf are the uses a product's set-up offers: none for a product
// that leaves the question out, its own when it gives some, and the
// set-up's own otherwise.
func setupUsesOf(product profile.Profile) []SetupUse {
	if product.SetupAsksNoUse {
		return []SetupUse{}
	}
	if len(product.SetupUses) == 0 {
		return slices.Clone(setupUses)
	}
	out := make([]SetupUse, 0, len(product.SetupUses))
	for _, u := range product.SetupUses {
		out = append(out, SetupUse{ID: u.ID, Label: u.Label, Windows: slices.Clone(u.Windows), Opens: u.Opens})
	}
	return out
}

// installPage is the page that says how to install a provider's program:
// the product's own for it, or its maker's.
func installPage(provider string, p profile.Profile) string {
	if page := p.InstallPages[provider]; page != "" {
		return page
	}
	return providers.Registry[provider].InstallPage
}

// SetupOffer reads what the set-up asks about. It looks for every agent's
// program, which takes a moment.
func (a *App) SetupOffer() (SetupOffer, error) {
	a.mu.Lock()
	offer := SetupOffer{Shown: a.settings.SetupShown, Default: defaultProviderOf(a.settings), Workspace: a.settings.Workspace, JudgeCalls: judgementStartCalls}
	a.mu.Unlock()
	offer.Uses = offeredSetupUses()
	offer.PersonName, offer.AccountName = a.chosenPersonName(), accountName()
	for _, p := range a.AgentProviders() {
		m := providerModules[p.ID]
		offer.Agents = append(offer.Agents, SetupAgent{ID: p.ID, Name: p.Name, Found: p.Available, Words: p.StatusWords,
			SignIn: providers.Registry[p.ID].SignIn && m.SignIn != nil, InstallPage: installPage(p.ID, product) != "", Judges: m.Judge != nil})
	}
	if ids := judgeProviders(); len(ids) > 0 {
		offer.JudgeName = providerName(ids[0])
	}
	snap, err := a.JudgementSettings()
	if err != nil {
		return offer, err
	}
	offer.JudgeOn = snap.Settings.Setup == "on" && snap.Settings.Enabled
	return offer, nil
}

// SetupShown says the set-up has opened by itself already.
func (a *App) SetupShown() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.settings.SetupShown
}

// NoteSetupShown records that the set-up opened by itself, which it does
// once. The judgements' own question is asked in the set-up, so its flyout
// does not open by itself as well.
func (a *App) NoteSetupShown() error {
	if err := a.updateSettings(func(s *Settings) error {
		s.SetupShown = true
		return nil
	}); err != nil {
		return err
	}
	return a.NoteJudgementSetupShown()
}

// OpenInstallPage opens the page that says how to install a provider's
// program, in the person's browser.
func (a *App) OpenInstallPage(provider string) error {
	page := installPage(normalizedProvider(provider), product)
	if !product.Offers(provider) || page == "" {
		return errors.New("there is no page that says how to install that agent")
	}
	return a.openInBrowser(page)
}

// askingCategory is the category the protocol gives the setting that says
// how a session asks before it acts (its session modes).
const askingCategory = "mode"

// askingOption is the option of a provider's own catalog that says what it
// asks before it acts, when its catalog is known and has one.
func (a *App) askingOption(provider string) (acp.ConfigOption, json.RawMessage, bool) {
	a.mu.Lock()
	prefs := a.modelPreferencesLocked(provider)
	a.mu.Unlock()
	for _, o := range prefs.Catalog {
		if o.Category == askingCategory && len(judgeOptionValues(o)) > 0 {
			return o, prefs.Defaults[o.ID], true
		}
	}
	return acp.ConfigOption{}, nil, false
}

// SetupAsking is a provider's setting for what it asks before it acts, or
// nil for a provider that has none the set-up can offer: one with no such
// setting, or one whose options are not known until its first chat.
func (a *App) SetupAsking(provider string) *SetupAsking {
	provider = normalizedProvider(provider)
	if !product.Offers(provider) {
		return nil
	}
	if m := providerModules[provider].Asking; m != nil {
		a.mu.Lock()
		current := m.Current(a.settings)
		a.mu.Unlock()
		return &SetupAsking{Choices: slices.Clone(m.Choices), Current: current}
	}
	option, saved, ok := a.askingOption(provider)
	if !ok {
		return nil
	}
	asking := &SetupAsking{Current: providers.Registry[provider].NewChatOptions[option.ID]}
	if len(saved) > 0 {
		asking.Current = judgeOptionString(saved)
	}
	for _, v := range judgeOptionValues(option) {
		label := v.Name
		if label == "" {
			label = v.Value
		}
		asking.Choices = append(asking.Choices, SetupChoice{ID: v.Value, Label: label})
	}
	return asking
}

// SetupChoices are the answers the set-up sets. Asking is empty for an agent
// with no such setting, Folder is empty when no folder was chosen, and
// SetName says the name was answered: an empty Name then means the
// account's.
type SetupChoices struct {
	Agent   string `json:"agent"`
	Asking  string `json:"asking"`
	Folder  string `json:"folder"`
	Name    string `json:"name"`
	SetName bool   `json:"setName"`
	Judge   bool   `json:"judge"`
}

// setupBefore is what the settings were before the set-up set them, with
// which of them it set, so that Undo puts back those and no others.
type setupBefore struct {
	provider    string
	providerSet bool
	// asking is the agent whose setting was set; askingOption is the option
	// of its catalog, or empty for a setting of its own module, and
	// askingValue what it had.
	asking       string
	askingOption string
	askingValue  string
	askingRaw    json.RawMessage
	name         string
	nameSet      bool
	// workspace is the one in front before, by ID, and added the one the
	// set-up made, or empty.
	workspace    string
	workspaceSet bool
	added        string
	judge        bool
}

// setupUndo holds what the last set-up set, until it is undone or the
// window ends.
var setupUndo struct {
	sync.Mutex
	before *setupBefore
}

// ApplySetup sets what the person answered. A step that fails stops it, and
// what was set before that step is put back, so that a failed set-up leaves
// the settings as they were.
func (a *App) ApplySetup(c SetupChoices) error {
	c.Agent = normalizedProvider(c.Agent)
	if !product.Offers(c.Agent) {
		return errors.New("choose an agent first")
	}
	setupUndo.Lock()
	defer setupUndo.Unlock()
	before := &setupBefore{}
	fail := func(what string, err error) error {
		if undo := a.undoSetup(before); undo != nil {
			return fmt.Errorf("%s: %v. Putting the rest back also failed: %v", what, err, undo)
		}
		return fmt.Errorf("%s: %v. Nothing was changed", what, err)
	}
	a.mu.Lock()
	before.provider = a.settings.DefaultProvider
	if i := workspaces.IndexAt(a.settings.WorkspaceRecords, a.settings.Workspace); a.settings.Workspace != "" && i >= 0 {
		before.workspace = a.settings.WorkspaceRecords[i].ID
	}
	known := len(a.settings.WorkspaceRecords)
	a.mu.Unlock()

	if err := a.SetDefaultProvider(c.Agent); err != nil {
		return fail("The agent could not be set", err)
	}
	before.providerSet = true
	if c.Asking != "" {
		if err := a.setSetupAsking(c.Agent, c.Asking, before); err != nil {
			return fail("What the agent asks could not be set", err)
		}
	}
	if c.SetName {
		before.name = a.chosenPersonName()
		if err := a.SetPersonName(c.Name); err != nil {
			return fail("The name could not be set", err)
		}
		before.nameSet = true
	}
	if c.Folder != "" {
		if _, err := a.SetWorkspace(c.Folder); err != nil {
			return fail("The folder could not be made the workspace", err)
		}
		before.workspaceSet = true
		a.mu.Lock()
		if i := workspaces.IndexAt(a.settings.WorkspaceRecords, filepath.Clean(c.Folder)); i >= 0 && len(a.settings.WorkspaceRecords) > known {
			before.added = a.settings.WorkspaceRecords[i].ID
		}
		a.mu.Unlock()
	}
	if c.Judge {
		if err := a.AcceptJudgementSetup(); err != nil {
			return fail("The small questions in the background could not be turned on", err)
		}
		before.judge = true
	} else if err := a.DeferJudgementSetup(); err != nil {
		return fail("The answer about the small questions could not be saved", err)
	}
	setupUndo.before = before
	return nil
}

// setSetupAsking saves what an agent asks before it acts, and notes what it
// had.
func (a *App) setSetupAsking(provider, value string, before *setupBefore) error {
	asking := a.SetupAsking(provider)
	if asking == nil || !slices.ContainsFunc(asking.Choices, func(c SetupChoice) bool { return c.ID == value }) {
		return errors.New("the agent does not offer that choice")
	}
	if m := providerModules[provider].Asking; m != nil {
		if err := m.Set(a, value); err != nil {
			return err
		}
		before.asking, before.askingValue = provider, asking.Current
		return nil
	}
	option, saved, _ := a.askingOption(provider)
	raw, _ := json.Marshal(value)
	if _, err := a.SetDefaultForProvider(provider, option.ID, raw); err != nil {
		return err
	}
	before.asking, before.askingOption, before.askingRaw = provider, option.ID, saved
	return nil
}

// UndoSetup puts back what the last set-up set. It does not sign anybody
// out, and it turns the small questions in the background off, since their
// question cannot be made unanswered.
func (a *App) UndoSetup() error {
	setupUndo.Lock()
	defer setupUndo.Unlock()
	if setupUndo.before == nil {
		return errors.New("there is no set-up to undo")
	}
	err := a.undoSetup(setupUndo.before)
	setupUndo.before = nil
	return err
}

// undoSetup puts back each thing the record says was set, and reports every
// one that could not be put back.
func (a *App) undoSetup(b *setupBefore) error {
	var failed []string
	note := func(what string, err error) {
		if err != nil {
			failed = append(failed, what+": "+err.Error())
		}
	}
	if b.judge {
		snap, err := a.JudgementSettings()
		if err == nil {
			s := snap.Settings
			s.Enabled = false
			_, err = a.SetJudgementSettings(s, snap.Revision)
		}
		note("the small questions in the background", err)
	}
	if b.workspaceSet {
		if b.added != "" {
			_, err := a.RemoveWorkspace(b.added)
			note("the workspace", err)
		}
		if b.workspace != "" {
			_, err := a.SelectWorkspace(b.workspace)
			note("the workspace in front", err)
		}
	}
	if b.nameSet {
		note("the name", a.SetPersonName(b.name))
	}
	if b.asking != "" {
		if b.askingOption == "" {
			note("what the agent asks", providerModules[b.asking].Asking.Set(a, b.askingValue))
		} else {
			raw := b.askingRaw
			if len(raw) == 0 {
				raw = json.RawMessage("null")
			}
			_, err := a.SetDefaultForProvider(b.asking, b.askingOption, raw)
			note("what the agent asks", err)
		}
	}
	if b.providerSet {
		// The saved value is put back as it was, which may be none: the
		// product's own default then stands again.
		err := a.updateSettings(func(s *Settings) error {
			s.DefaultProvider = b.provider
			return nil
		})
		if err == nil {
			a.emitSettingsChanged()
			a.emitProviderFacts()
		}
		note("the agent", err)
	}
	if len(failed) > 0 {
		return errors.New(strings.Join(failed, "; "))
	}
	return nil
}

// SetupQuestion is one open question as the reading of a person's own words
// is given it: its ID (use, ask, folder, name or judge) and its choices,
// where it has a list of them.
type SetupQuestion struct {
	ID      string        `json:"id"`
	Choices []SetupChoice `json:"choices,omitempty"`
}

// SetupReading is what was taken from a person's own words: an answer for
// each question the words answer, by the question's ID. A choice is its ID,
// a folder its path, a name the name, and judge is "on" or "off". Used says
// who read them: "model" for the chosen agent, "rule" for word matching.
type SetupReading struct {
	Answers map[string]string `json:"answers"`
	Used    string            `json:"used"`
}

// setupReadTimeout is how long the chosen agent may take to read a
// person's words before word matching reads them.
const setupReadTimeout = 25 * time.Second

const setupReadSystem = `You are reading what a person typed during the set-up of a desktop program, to take from it the answers to the set-up's open questions.
The input has "words", which the person typed, and "questions", the open ones. A question with "choices" is answered with the id of one of its choices. The question "folder" is answered with a folder path the person wrote, exactly as written. The question "name" is answered with what the person asks to be called. The question "judge" asks whether an agent may answer small questions in the background, and is answered "on" or "off".
Answer only the questions the words clearly answer, and leave the others out. Answer with one JSON object whose keys are question ids and whose values are strings, such as {"use":"docs","folder":"C:\\Work"}. Answer {} when the words answer none.`

// setupRead asks the chosen agent; tests replace it.
var setupRead = func(a *App, ctx context.Context, provider, input string) (string, error) {
	m := providerModules[provider].Judge
	if m == nil {
		return "", errors.New("the agent cannot read free text here")
	}
	res, err := m.Build(a, provider, "").Ask(ctx, judge.Request{System: setupReadSystem, Input: input, JSON: true, MaxTokens: 300})
	return res.Text, err
}

// ReadSetupWords takes answers to the open questions from what the person
// typed. The chosen agent reads the words, in a session of its own with no
// tools, as a judgement call is made; judgements themselves are not on yet
// at this point, so the call is made directly and is not counted in their
// budget. When the agent does not answer, or answers nothing that can be
// used, the words are matched instead.
func (a *App) ReadSetupWords(words, provider string, open []SetupQuestion) (SetupReading, error) {
	words = strings.TrimSpace(words)
	if words == "" || len([]rune(words)) > 2000 {
		return SetupReading{}, errors.New("say it in up to 2,000 characters")
	}
	provider = normalizedProvider(provider)
	input, _ := json.Marshal(struct {
		Words     string          `json:"words"`
		Questions []SetupQuestion `json:"questions"`
	}{words, open})
	ctx, cancel := context.WithTimeout(context.Background(), setupReadTimeout)
	defer cancel()
	if text, err := setupRead(a, ctx, provider, string(input)); err == nil {
		if answers := checkedSetupAnswers(modelSetupAnswers(text), open); len(answers) > 0 {
			return SetupReading{Answers: answers, Used: "model"}, nil
		}
	} else {
		logf("core", "info", "the set-up's words were read by matching: %v", err)
	}
	return SetupReading{Answers: checkedSetupAnswers(setupRule(words, open), open), Used: "rule"}, nil
}

// modelSetupAnswers reads the object in a model's answer, which may have
// words or a fence around it.
func modelSetupAnswers(text string) map[string]string {
	start, end := strings.Index(text, "{"), strings.LastIndex(text, "}")
	if start < 0 || end < start {
		return nil
	}
	var raw map[string]any
	if json.Unmarshal([]byte(text[start:end+1]), &raw) != nil {
		return nil
	}
	out := map[string]string{}
	for k, v := range raw {
		if s, ok := v.(string); ok {
			out[k] = strings.TrimSpace(s)
		}
	}
	return out
}

// checkedSetupAnswers keeps the answers that an open question can take: a
// choice the question offers, a folder that exists, a name that fits, and on
// or off.
func checkedSetupAnswers(answers map[string]string, open []SetupQuestion) map[string]string {
	out := map[string]string{}
	for _, q := range open {
		v, ok := answers[q.ID]
		if !ok || v == "" {
			continue
		}
		switch {
		case len(q.Choices) > 0:
			if !slices.ContainsFunc(q.Choices, func(c SetupChoice) bool { return c.ID == v }) {
				continue
			}
		case q.ID == "folder":
			if info, err := os.Stat(v); err != nil || !info.IsDir() || !filepath.IsAbs(v) {
				continue
			}
		case q.ID == "name":
			if len([]rune(v)) > personNameLimit {
				continue
			}
		case q.ID == "judge":
			if v != "on" && v != "off" {
				continue
			}
		default:
			continue
		}
		out[q.ID] = v
	}
	return out
}

var (
	setupPathWords = regexp.MustCompile(`(?i)(?:[a-z]:\\|\\\\|/)[^\s,;"]+(?:[\\/][^\s,;"]+)*`)
	setupNameWords = regexp.MustCompile(`(?i)\b(?:call me|my name is|i am called)\s+([\pL][\pL\pN'-]*(?:\s+[\pLu][\pL\pN'-]*)?)`)
	setupYesWords  = regexp.MustCompile(`(?i)\bbackground\b.*\b(yes|fine|ok|okay|on)\b|\b(yes|fine|ok|okay)\b.*\bbackground\b`)
	setupNoWords   = regexp.MustCompile(`(?i)\bbackground\b.*\b(no|not|off)\b|\bno\b.*\bbackground\b`)
)

// setupUseWords are words that say one of the set-up's own uses.
var setupUseWords = []struct {
	id    string
	words *regexp.Regexp
}{
	{"team", regexp.MustCompile(`(?i)\bteams?\b|several agents|agents together`)},
	{"docs", regexp.MustCompile(`(?i)\b(documents?|requirements?|writing|diagrams?|specs?|specifications?)\b`)},
	{"code", regexp.MustCompile(`(?i)\b(code|coding|programs?|programming|develop\w*|repos?|repository)\b`)},
}

// setupRule takes what it can from the words by matching them: a choice
// whose own label the words have in them, one of the set-up's own uses by
// its usual words, a folder path, a name after "call me", and yes or no to
// the background.
func setupRule(words string, open []SetupQuestion) map[string]string {
	out := map[string]string{}
	lower := strings.ToLower(words)
	for _, q := range open {
		for _, c := range q.Choices {
			if strings.Contains(lower, strings.ToLower(c.Label)) {
				out[q.ID] = c.ID
				break
			}
		}
		switch q.ID {
		case "use":
			if _, done := out[q.ID]; done {
				continue
			}
			for _, u := range setupUseWords {
				if u.words.MatchString(words) {
					out[q.ID] = u.id
					break
				}
			}
		case "folder":
			if path := strings.TrimRight(setupPathWords.FindString(words), "."); path != "" {
				out[q.ID] = path
			}
		case "name":
			if m := setupNameWords.FindStringSubmatch(words); m != nil {
				out[q.ID] = m[1]
			}
		case "judge":
			if setupNoWords.MatchString(words) {
				out[q.ID] = "off"
			} else if setupYesWords.MatchString(words) {
				out[q.ID] = "on"
			}
		}
	}
	return out
}
