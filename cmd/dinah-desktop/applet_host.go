package main

import (
	"context"
	"encoding/base64"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
	"studio/internal/applet"
	"studio/internal/applet/broker"
	"studio/internal/applet/host"
)

// The app invokes applet actions through package broker, as the command line
// does, with its invocations in the app's state folder and every change told
// to the page.

type (
	AppletInvocation = broker.Invocation
	AppletArtifact   = broker.Artifact
)

var errAppletRecovery = broker.ErrRecovery

func (a *App) appletHostService() *host.Service {
	a.appletMu.Lock()
	defer a.appletMu.Unlock()
	if a.appletHost == nil {
		a.appletHost = host.New(host.Options{
			State: configDir(), Builtins: a.builtinAppletBundles,
			Extensions: func() ([]applet.Bundle, error) { return a.extensionManager().Bundles() },
			Revision:   func() string { return a.extensionService().LibraryRevision() },
			Emit:       func(name string, data any) { logAppletEvent(name, data); a.emit(name, data) },
			Consent:    a.appletConsent,
			Log:        func(level, format string, args ...any) { logf("core", level, format, args...) },
			Logf:       logf,
		})
	}
	return a.appletHost
}

func (a *App) appletRegistry() (*applet.Registry, error) {
	b, err := a.appletHostService().Broker()
	if err != nil {
		return nil, err
	}
	return b.Registry, nil
}

// appletStore is the broker's view of the saved invocations, which needs no
// registry.
func appletStore() *broker.Broker { return &broker.Broker{State: configDir()} }

func appletStoreRoot(workspace string) string { return appletStore().StoreRoot(workspace) }

func readAppletInvocation(workspace, id string) (AppletInvocation, error) {
	return appletStore().Read(workspace, id)
}

func saveAppletInvocation(v *AppletInvocation) error { return appletStore().Save(v) }

func newAppletID() (string, error) { return broker.NewID() }

func (a *App) startAppletInvocation(workspace, ownerKind, ownerID, ref, inputJSON, expectedDigest string, deadline time.Time) (AppletInvocation, error) {
	b, err := a.appletHostService().Broker()
	if err != nil {
		return AppletInvocation{}, err
	}
	return b.Start(workspace, ownerKind, ownerID, ref, inputJSON, expectedDigest, deadline)
}

func (a *App) startAppletInvocationID(workspace, ownerKind, ownerID, id, ref, inputJSON, expectedDigest string, deadline time.Time) (AppletInvocation, error) {
	b, err := a.appletHostService().Broker()
	if err != nil {
		return AppletInvocation{}, err
	}
	return b.StartID(workspace, ownerKind, ownerID, id, ref, inputJSON, expectedDigest, deadline)
}

func (a *App) appletInvocation(workspace, ownerKind, ownerID, id string) (AppletInvocation, error) {
	b, err := a.appletHostService().Broker()
	if err != nil {
		return AppletInvocation{}, err
	}
	return b.Get(workspace, ownerKind, ownerID, id)
}

func (a *App) cancelAppletInvocation(workspace, ownerKind, ownerID, id string) error {
	b, err := a.appletHostService().Broker()
	if err != nil {
		return err
	}
	return b.Cancel(workspace, ownerKind, ownerID, id)
}

func (a *App) runAppletAction(ctx context.Context, workspace, ownerKind, ownerID, id, ref, inputJSON, expectedDigest string, deadline time.Time) (string, error) {
	b, err := a.appletHostService().Broker()
	if err != nil {
		return "", err
	}
	return b.Run(ctx, workspace, ownerKind, ownerID, id, ref, inputJSON, expectedDigest, deadline)
}

func (a *App) appletArtifact(workspace, ownerKind, ownerID, id string) (AppletArtifact, error) {
	b, err := a.appletHostService().Broker()
	if err != nil {
		return AppletArtifact{}, err
	}
	return b.Artifact(workspace, ownerKind, ownerID, id)
}

func (a *App) AppletCatalog() ([]applet.Manifest, error) {
	r, err := a.appletRegistry()
	if err != nil {
		return nil, err
	}
	return r.Catalog(), nil
}

// InvokeAppletAt is the trusted human UI adapter. Agent calls use their bound channel.
func (a *App) InvokeAppletAt(workspace, provider, ref, inputJSON string) (AppletInvocation, error) {
	if !strings.HasPrefix(ref, provider+"/") {
		return AppletInvocation{}, errors.New("the action belongs to a different applet")
	}
	return a.startAppletInvocation(workspace, "applet", provider, ref, inputJSON, "", time.Now().Add(24*time.Hour))
}

func (a *App) AppletInvocationsAt(workspace string) ([]AppletInvocation, error) {
	b, err := a.appletHostService().Broker()
	if err != nil {
		return nil, err
	}
	return b.List(workspace)
}

func (a *App) AnswerAppletRequest(workspace, id string, version int, responseJSON string) (AppletInvocation, error) {
	b, err := a.appletHostService().Broker()
	if err != nil {
		return AppletInvocation{}, err
	}
	return b.Answer(workspace, id, version, responseJSON)
}

func (a *App) AppletArtifactAt(workspace, id string) (AppletArtifact, error) {
	split := strings.LastIndex(id, "_")
	if split < 1 {
		return AppletArtifact{}, errors.New("invalid artifact identity")
	}
	v, err := readAppletInvocation(workspace, id[:split])
	if err != nil {
		return AppletArtifact{}, err
	}
	return a.appletArtifact(workspace, v.OwnerKind, v.OwnerID, id)
}

func (a *App) AppletInvocationAt(workspace, id string) (AppletInvocation, error) {
	v, err := readAppletInvocation(workspace, id)
	if err != nil {
		return AppletInvocation{}, err
	}
	return a.appletInvocation(workspace, v.OwnerKind, v.OwnerID, id)
}

func (a *App) CancelAppletInvocationAt(workspace, id string) error {
	v, err := readAppletInvocation(workspace, id)
	if err != nil {
		return err
	}
	return a.cancelAppletInvocation(workspace, v.OwnerKind, v.OwnerID, id)
}

// An applet invocation has an entry in Notifications only when nothing else
// shows a person what it needs them to see.
//
//   - A question, an invocation waiting for a person's answer, is always
//     one: only a person can answer it, and Notifications is where they do.
//   - A notification a script posts on purpose
//     (dinah.notifications/post) is one until it is dismissed, since
//     posting it is asking for an entry.
//   - Artifacts a chat made are one until dismissed, since a chat window
//     cannot open an artifact. An applet window that starts an action shows
//     its own.
//
// No other result is one, whether it succeeded or failed. The caller always
// has it: the Terminal prints a result and its error, a chat shows the tool
// call and its error, and an applet window shows what it started and lists
// its invocations.

func appletQuestion(s broker.Summary) bool { return s.Status == "waiting" }

func appletNotice(s broker.Summary) bool {
	if s.Status != "done" || s.Dismissed {
		return false
	}
	return postsNotification(s.Action) || (s.Artifacts > 0 && s.OwnerKind != "applet")
}

func postsNotification(action string) bool {
	return action == "dinah.notifications/post@1"
}

// AppletAttentionLists is what Notifications shows of applet invocations: the
// questions waiting for a person and the results that need one, from one
// pass over the saved invocations.
type AppletAttentionLists struct {
	Requests []AppletInvocation `json:"requests"`
	Results  []AppletInvocation `json:"results"`
}

func (a *App) AppletAttention() (AppletAttentionLists, error) {
	b, err := a.appletHostService().Broker()
	if err != nil {
		return AppletAttentionLists{}, err
	}
	list, err := b.Attention(func(s broker.Summary) bool { return appletQuestion(s) || appletNotice(s) })
	if err != nil {
		return AppletAttentionLists{}, err
	}
	out := AppletAttentionLists{Requests: []AppletInvocation{}, Results: []AppletInvocation{}}
	for _, v := range list {
		if appletQuestion(broker.Summarize(v)) {
			out.Requests = append(out.Requests, v)
		} else {
			out.Results = append(out.Results, v)
		}
	}
	return out, nil
}

// AppletRequests lists the questions of AppletAttention.
func (a *App) AppletRequests() ([]AppletInvocation, error) {
	b, err := a.appletHostService().Broker()
	if err != nil {
		return nil, err
	}
	return b.Attention(appletQuestion)
}

// AppletResults lists the results of AppletAttention.
func (a *App) AppletResults() ([]AppletInvocation, error) {
	b, err := a.appletHostService().Broker()
	if err != nil {
		return nil, err
	}
	return b.Attention(appletNotice)
}

func (a *App) DismissAppletResultAt(workspace, id string) error {
	b, err := a.appletHostService().Broker()
	if err != nil {
		return err
	}
	return b.Dismiss(workspace, id)
}

func (a *App) SaveAppletArtifactAt(workspace, id string) (string, error) {
	artifact, err := a.AppletArtifactAt(workspace, id)
	if err != nil {
		return "", err
	}
	if artifact.MIME != "image/svg+xml" {
		return "", errors.New("native export currently supports SVG artifacts; copy text from the applet")
	}
	data, err := base64.StdEncoding.DecodeString(artifact.DataBase64)
	if err != nil {
		return "", errors.New("the artifact is not valid base64")
	}
	name := filepath.Base(artifact.Name)
	if name == "." || name == "" {
		name = "artifact.svg"
	}
	if !strings.EqualFold(filepath.Ext(name), ".svg") {
		name += ".svg"
	}
	path, err := wruntime.SaveFileDialog(a.ctx, wruntime.SaveDialogOptions{Title: "Save Artifact", DefaultFilename: name, Filters: []wruntime.FileFilter{{DisplayName: "SVG Image (*.svg)", Pattern: "*.svg"}}})
	if err != nil || path == "" {
		return "", err
	}
	return path, os.WriteFile(path, data, 0o600)
}
