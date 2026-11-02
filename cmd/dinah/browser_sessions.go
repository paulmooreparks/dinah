package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"

	"studio/internal/bus"
)

func browserSessions(ctx context.Context, b *bus.Bus, args []string, out io.Writer) error {
	if len(args) == 1 && args[0] == "sign-out-everywhere" {
		if err := b.RevokeAllWebTokens(ctx); err != nil {
			return err
		}
		_, err := fmt.Fprintln(out, "All browser sign-ins and unused sign-in links have been revoked.")
		return err
	}
	if len(args) == 0 || args[0] != "settings" {
		return errors.New("choose settings or sign-out-everywhere")
	}
	p, err := b.WebSessionPolicy(ctx)
	if err != nil {
		return err
	}
	flags := flag.NewFlagSet("browser-sessions settings", flag.ContinueOnError)
	flags.SetOutput(out)
	idle := flags.Int("idle-days", p.IdleDays, "days since last active use before sign-in expires (1 to 3650)")
	absolute := flags.Int("absolute-days", p.AbsoluteDays, "days since issue before sign-in expires (1 to 3650)")
	if err := flags.Parse(args[1:]); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("browser session settings accept named flags only")
	}
	if flags.NFlag() > 0 {
		p = bus.WebSessionPolicy{IdleDays: *idle, AbsoluteDays: *absolute}
		if err := b.SetWebSessionPolicy(ctx, p); err != nil {
			return err
		}
	}
	_, err = fmt.Fprintf(out, "Idle lifetime: %d days\nAbsolute lifetime: %d days\n", p.IdleDays, p.AbsoluteDays)
	return err
}
