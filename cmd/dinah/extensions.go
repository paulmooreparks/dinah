package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	extension "github.com/paulmooreparks/dinah-extension-go"
	"os"
	"path/filepath"
	"strings"
	"studio/internal/bus"
	"studio/internal/extensions"
)

func extensionCommand(ctx context.Context, args []string) error {
	home, err := bus.Home()
	if err != nil {
		return err
	}
	store := extension.Store{Root: filepath.Join(home, "extensions")}
	if len(args) == 0 {
		return errors.New("usage: " + product.CommandProgram + " ext install SOURCE | update [ID] | list | enable ID | disable ID | remove ID")
	}
	switch args[0] {
	case "list":
		list, err := store.List()
		if err != nil {
			return err
		}
		for _, v := range list {
			state := "enabled"
			if !v.Enabled {
				state = "disabled"
			}
			fmt.Printf("%s %s (%s) %s\n", v.Manifest.ID, v.Manifest.Version, state, v.Origin)
		}
		return nil
	case "install":
		flags := flag.NewFlagSet("ext install", flag.ContinueOnError)
		accept := flags.String("accept-key", "", "approve this exact publisher fingerprint for this installation")
		if err = flags.Parse(args[1:]); err != nil {
			return err
		}
		if flags.NArg() != 1 {
			return errors.New("usage: " + product.CommandProgram + " ext install [--accept-key FINGERPRINT] SOURCE")
		}
		return installExtensionSource(ctx, store, flags.Arg(0), *accept)
	case "update":
		if len(args) > 2 {
			return errors.New("usage: " + product.CommandProgram + " ext update [ID]")
		}
		list, err := store.List()
		if err != nil {
			return err
		}
		found := false
		for _, v := range list {
			if len(args) == 2 && v.Manifest.ID != args[1] {
				continue
			}
			found = true
			if err = installExtensionSource(ctx, store, v.Origin, ""); err != nil {
				return err
			}
		}
		if !found {
			return errors.New("no matching installed extension")
		}
		return nil
	case "enable", "disable":
		if len(args) != 2 {
			return errors.New("an extension ID is required")
		}
		return extensions.WithPausedHosts(ctx, store.Root, args[1], func() error { return store.Enable(args[1], args[0] == "enable") })
	case "remove":
		if len(args) != 2 {
			return errors.New("an extension ID is required")
		}
		fmt.Printf("Remove %s and all its extension data? Type the ID to confirm: ", args[1])
		answer, _ := bufio.NewReader(os.Stdin).ReadString('\n')
		if strings.TrimSpace(answer) != args[1] {
			return errors.New("removal cancelled")
		}
		return extensions.WithPausedHosts(ctx, store.Root, args[1], func() error { return store.Remove(args[1]) })
	}
	return errors.New("unknown extension command")
}
func installExtensionSource(ctx context.Context, store extension.Store, source, accept string) error {
	folder, cleanup, err := extensions.PrepareSource(ctx, source)
	if err != nil {
		return err
	}
	defer cleanup()
	preview, err := store.Inspect(folder)
	if err != nil {
		return err
	}
	fmt.Printf("%s %s\nPublisher: %s\nPublisher key SHA-256: %s\nPermissions: %s\nOrigin: %s\nNative code runs under your account. Host permissions do not sandbox the native process.\n", preview.Manifest.Title, preview.Manifest.Version, preview.Manifest.Publisher.Name, preview.Fingerprint, strings.Join(preview.Manifest.Permissions, ", "), source)
	if accept == "" {
		fmt.Print("Install this signed package? Type yes: ")
		answer, _ := bufio.NewReader(os.Stdin).ReadString('\n')
		if strings.TrimSpace(answer) != "yes" {
			return errors.New("installation cancelled")
		}
	} else if accept != preview.Fingerprint {
		return errors.New("publisher fingerprint did not match --accept-key")
	}
	err = extensions.WithPausedHosts(ctx, store.Root, preview.Manifest.ID, func() error {
		_, installErr := store.Install(folder, source, extension.Approval{Fingerprint: preview.Fingerprint, Signature: preview.Manifest.Signature})
		return installErr
	})
	if err == nil {
		fmt.Printf("Installed. Open Extensions in %s and refresh its catalog.\n", product.Name)
	}
	return err
}
