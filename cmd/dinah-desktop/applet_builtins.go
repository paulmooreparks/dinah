package main

import (
	"embed"
	"io/fs"

	"studio/internal/applet"
	"studio/internal/applet/builtin"
	"studio/internal/applet/host"
)

//go:embed applet_builtins.go applet_host.go
var builtinAppletSources embed.FS

// builtinAppletBundles are the built-in applets: those every program offers,
// from package builtin as they are, those a product built from this source
// registered (applet.RegisterBundle), and the app's own, whose
// implementation includes the app's host files.
func (a *App) builtinAppletBundles() ([]applet.Bundle, error) {
	bundles, err := builtin.Bundles()
	if err != nil {
		return nil, err
	}
	product, err := applet.ProductBundles()
	if err != nil {
		return nil, err
	}
	bundles = append(bundles, product...)
	makers := []func() (applet.Bundle, error){a.workspaceAppletBundle, a.chatAppletBundle, a.settingsAppletBundle, a.filesAppletBundle, a.terminalAppletBundle, a.skillsAppletBundle, a.windowsAppletBundle, a.teamAppletBundle, a.diagnosticsAppletBundle, a.documentsAppletBundle, a.documentThreadsAppletBundle}
	makers = append(makers, a.standaloneAppletBundle)
	// The bundle's two actions, dinah.requests/withdraw@1 and
	// dinah.requests/withdrawal-outcome@1, let a chat withdraw what it asked
	// through the applet actions as well as through the bus tools
	// withdraw_request and request_operation. Paul ruled on 17 November 2026
	// that agents have both; the actions are the only way an agent can
	// withdraw a task question or a report.
	makers = append(makers, a.requestSettlementBundle)
	for _, app := range contentApps {
		app := app
		makers = append(makers, func() (applet.Bundle, error) { return a.contentAppletBundle(app) })
	}
	for _, makeBundle := range makers {
		bundle, err := makeBundle()
		if err != nil {
			return nil, err
		}
		if bundle.Sources == nil {
			bundle.Sources = map[string][]byte{}
		}
		bundle.Sources["skill_catalog.go"] = skillCatalogSource
		bundle.Sources["host/service.go"] = host.Source
		err = fs.WalkDir(builtinAppletSources, ".", func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil || entry.IsDir() {
				return walkErr
			}
			data, err := builtinAppletSources.ReadFile(path)
			bundle.Sources["host/"+path] = data
			return err
		})
		if err != nil {
			return nil, err
		}
		bundles = append(bundles, bundle)
	}
	return bundles, nil
}
