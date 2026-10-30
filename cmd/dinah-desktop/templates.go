package main

import (
	"errors"
	"net/url"
	"strings"
)

// Templates returns the person's saved prompts in the order they were saved.
func (a *App) Templates() []Template {
	a.mu.Lock()
	defer a.mu.Unlock()
	return append([]Template(nil), a.settings.Templates...)
}

// SaveTemplate keeps a prompt under a name, replacing a template of the same
// name, compared without regard to case.
func (a *App) SaveTemplate(name, text string) ([]Template, error) {
	name, text = strings.TrimSpace(name), strings.TrimSpace(text)
	if name == "" || text == "" {
		return a.Templates(), errors.New("a template needs a name and some text")
	}
	var out []Template
	err := a.updateSettings(func(s *Settings) error {
		out = []Template{}
		replaced := false
		for _, t := range s.Templates {
			if strings.EqualFold(t.Name, name) {
				t, replaced = Template{Name: name, Text: text}, true
			}
			out = append(out, t)
		}
		if !replaced {
			out = append(out, Template{Name: name, Text: text})
		}
		s.Templates = out
		return nil
	})
	if err != nil {
		return a.Templates(), err
	}
	return append([]Template(nil), out...), nil
}

// DeleteTemplate forgets the template of a name.
func (a *App) DeleteTemplate(name string) ([]Template, error) {
	var out []Template
	err := a.updateSettings(func(s *Settings) error {
		out = []Template{}
		for _, t := range s.Templates {
			if !strings.EqualFold(t.Name, name) {
				out = append(out, t)
			}
		}
		s.Templates = out
		return nil
	})
	if err != nil {
		return a.Templates(), err
	}
	return append([]Template(nil), out...), nil
}

// usagePage is the page that shows the person's Devin usage, or "" when the
// person has not given one.
func (a *App) usagePage() string {
	return a.settings.UsagePage
}

// SetUsagePage changes the usage page; an empty address clears it.
func (a *App) SetUsagePage(page string) error {
	page = strings.TrimSpace(page)
	if page != "" {
		u, err := url.Parse(page)
		if err != nil || u.Scheme != "https" || u.Host == "" {
			return errors.New("the usage page must be an https address")
		}
	}
	return a.updateSettings(func(s *Settings) error {
		s.UsagePage = page
		return nil
	})
}

// OpenUsage opens the usage page in the person's browser, when there is one.
func (a *App) OpenUsage() {
	a.mu.Lock()
	page := a.usagePage()
	a.mu.Unlock()
	if page == "" {
		return
	}
	a.openInBrowser(page)
}
