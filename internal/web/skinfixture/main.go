package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"os"
	"path/filepath"

	"studio/internal/bus"
	"studio/internal/profile"
	"studio/internal/web"
)

func main() {
	home, err := os.MkdirTemp("", "dinah-web-skins-")
	must(err)
	must(os.Setenv(profile.Current().HomeVar(), home))
	set(home, "standard", "light")
	b, err := bus.OpenAt(filepath.Join(home, "bus.db"))
	must(err)
	defer b.Close()
	ctx := context.Background()
	_, err = b.RegisterSession(ctx, "codex:skin-fixture", "C:/SkinFixture", "Skin Fixture Chat")
	must(err)
	room, err := b.CreateGroup(ctx, bus.Sender{}, "Skin Fixture Group", []string{"Skin Fixture Chat"})
	must(err)
	_, err = b.Send(ctx, bus.Sender{}, "#Skin Fixture Group", "A saved fixture message for rendered skin verification.", nil, 0)
	must(err)
	ts := httptest.NewUnstartedServer(nil)
	s, err := web.New(b, ts.Listener.Addr().String())
	must(err)
	ts.Config.Handler = s
	ts.Start()
	defer ts.Close()
	signin, err := s.SigninURL(ctx)
	must(err)
	encoder := json.NewEncoder(os.Stdout)
	must(encoder.Encode(map[string]string{"url": ts.URL, "signin": signin, "home": home, "room": room.Name}))
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		var command struct{ Skin, Mode string }
		must(json.Unmarshal(scanner.Bytes(), &command))
		if command.Skin == "quit" {
			return
		}
		set(home, command.Skin, command.Mode)
		must(encoder.Encode(map[string]string{"skin": command.Skin, "mode": command.Mode}))
	}
	must(scanner.Err())
}

func set(home, skin, mode string) {
	appearance, err := json.Marshal(map[string]any{"version": 1, "active": "mineral", "skin": skin, "themes": []any{}, "overrides": map[string]any{}})
	must(err)
	must(os.WriteFile(filepath.Join(home, "appearance.json"), appearance, 0600))
	state, err := json.Marshal(map[string]any{"version": 1, "values": map[string]string{"pudl-theme": mode}})
	must(err)
	must(os.WriteFile(filepath.Join(home, "ui-state.json"), state, 0600))
}

func must(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
