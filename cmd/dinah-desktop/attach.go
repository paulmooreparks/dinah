package main

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"studio/internal/acp"
	"studio/internal/bus"
	"studio/internal/ollamaagent"
	"studio/internal/sessionhost"
)

// Attachment is something the person added to a prompt: a mention (of a
// document or a web page), an applet
// reference, or a file pasted or dropped into the prompt with its bytes.
type Attachment = sessionhost.Attachment

// The largest text file embedded in a prompt as it is; a larger one is saved
// and linked instead.
const maxEmbeddedText = 200 << 10

// maxAttachmentFile is the largest file the person attaches, in a single
// chat or a group chat; the bus keeps the limit, since it stores the files
// of a group's messages.
const maxAttachmentFile = bus.MaxAttachmentFile

type PickedAttachment struct {
	Attachment
	Size int `json:"size"`
}

// PickAttachments reads only the files selected in the native dialog. It uses
// the same payload as pasted and dropped files so sending remains unchanged.
func (a *App) PickAttachments() ([]PickedAttachment, error) {
	paths, err := wruntime.OpenMultipleFilesDialog(a.ctx, wruntime.OpenDialogOptions{Title: "Choose Files to Attach", DefaultDirectory: a.currentWorkspace()})
	if err != nil {
		return nil, err
	}
	items := make([]PickedAttachment, 0, len(paths))
	for _, path := range paths {
		item, err := readPickedAttachment(path)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

func readPickedAttachment(path string) (PickedAttachment, error) {
	f, err := os.Open(path)
	if err != nil {
		return PickedAttachment{}, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return PickedAttachment{}, err
	}
	if !info.Mode().IsRegular() {
		return PickedAttachment{}, fmt.Errorf("%s is not a regular file", filepath.Base(path))
	}
	if info.Size() > maxAttachmentFile {
		return PickedAttachment{}, fmt.Errorf("%s is larger than 25 MB, so it was not attached", filepath.Base(path))
	}
	data, err := io.ReadAll(io.LimitReader(f, maxAttachmentFile+1))
	if err != nil {
		return PickedAttachment{}, err
	}
	if len(data) > maxAttachmentFile {
		return PickedAttachment{}, fmt.Errorf("%s is larger than 25 MB, so it was not attached", filepath.Base(path))
	}
	contentType := mime.TypeByExtension(strings.ToLower(filepath.Ext(path)))
	if contentType == "" {
		contentType = http.DetectContentType(data)
	}
	if parsed, _, err := mime.ParseMediaType(contentType); err == nil {
		contentType = parsed
	}
	return PickedAttachment{Attachment: Attachment{Kind: "file", Name: filepath.Base(path), MimeType: contentType, Data: base64.StdEncoding.EncodeToString(data)}, Size: len(data)}, nil
}

// blocks turns a prompt's text and attachments into content blocks. A link
// is a resource_link, which every agent takes. A pasted or dropped image is
// an image block where the agent takes images, and a text file is embedded
// where the agent takes embedded resources; anything else, and anything the
// agent does not take, is saved in the session's workspace under
// .dinah/attachments and linked, so that the agent reads it with its own
// tools (https://agentclientprotocol.com/protocol/content).
// An applet reference adds guidance built from its registered manifest.
//
// A message with more files than the chat's provider takes (file_limits.go)
// is refused, as the box refuses the file past the limit. A picture for a
// Claude chat whose longer side is over 2000 px is reduced first
// (picture_fit.go).
func (a *App) blocks(s *Session, text string, atts []Attachment) ([]acp.ContentBlock, error) {
	caps := a.promptCaps(s)
	provider := normalizedProvider(s.Provider())
	if limit, n := a.providerFileLimit(provider), countFiles(atts); limit > 0 && n > limit {
		return nil, tooManyFiles(provider, limit, n)
	}
	// A message of attachments alone has no words, and a text block must
	// carry its text (ACP's TextContent requires it; Text is omitted when
	// empty), so it gets none.
	var out []acp.ContentBlock
	if text != "" {
		out = append(out, acp.ContentBlock{Type: "text", Text: text})
	}
	atts = append([]Attachment{}, atts...)
	for i := 0; i < len(atts); i++ {
		at := atts[i]
		switch at.Kind {
		case "resource":
			// Something in a workspace, attached with +, goes as the link or
			// the file it comes down to.
			if at.Ref == nil {
				return nil, fmt.Errorf("%s has no place in a workspace", at.Name)
			}
			conv, err := a.resourceAttachment(*at.Ref, at.Name)
			if err != nil {
				return nil, fmt.Errorf("%s could not be attached: %v", at.Name, err)
			}
			atts = append(atts, conv)
		case "applet":
			registry, err := a.appletRegistry()
			if err != nil {
				return nil, err
			}
			found := false
			for _, manifest := range registry.Catalog() {
				if manifest.ID != at.Provider {
					continue
				}
				selection := struct {
					Provider string   `json:"provider"`
					Title    string   `json:"title"`
					Actions  []string `json:"actions"`
				}{Provider: manifest.ID, Title: manifest.Title}
				for _, action := range manifest.Actions {
					selection.Actions = append(selection.Actions, action.Ref)
				}
				data, err := json.Marshal(selection)
				if err != nil {
					return nil, err
				}
				out = append(out, acp.ContentBlock{Type: "text", Text: "The user selected this bundled applet for the message:\n" + string(data) + "\nUse applet_discover to inspect its current action schemas and implementation digests. Use applet_invoke for the requested operation, then applet_status and applet_artifact for its result. " + product.Name + " manages this internal MCP connection automatically. If the tools are unavailable, report a " + product.Name + " agent-integration failure. Do not ask the person to enable, configure or reconnect dinah-applets; there is no user connection control for it. Selection alone does not execute an action or grant additional permissions. Human requests must be completed by the person in Notifications."})
				found = true
				break
			}
			if !found {
				return nil, fmt.Errorf("selected applet %q is unavailable; choose an available applet before sending", at.Provider)
			}
		case "window":
			// A tool window the person chose: the agent drives it through the
			// Windows applet, as pressing its commands would, and reads its
			// data through the tool's own applet when it has one.
			if !windowToolID.MatchString(at.Window) {
				return nil, fmt.Errorf("%s is not a %s window", at.Name, product.Name)
			}
			text := "The user selected the " + product.Name + " window " + strconv.Quote(at.Name) + " (tool " + strconv.Quote(at.Window) + ") for the message. Drive it through the bundled applet dinah.windows: " +
				windowsListAction + " finds it among the open windows by its kind, " + windowOpenAction + " with this tool opens it when it is not open, " + windowCommandsAction + " lists its commands with their state, and " + windowRunAction + " runs one as pressing it would. Use applet_discover for the schemas and applet_invoke to call them."
			if at.Provider != "" {
				if registry, err := a.appletRegistry(); err == nil {
					for _, manifest := range registry.Catalog() {
						if manifest.ID == at.Provider {
							text += " The same tool's data can be read through the bundled applet " + manifest.ID + " (" + manifest.Title + ")."
							break
						}
					}
				}
			}
			out = append(out, acp.ContentBlock{Type: "text", Text: text + " Selection alone does not run anything or grant permissions."})
		case "chat":
			// Another open chat the person mentioned: the agent asks it
			// through the Chat applet.
			t := a.session(at.SessionID)
			a.mu.Lock()
			hidden := a.hidden(at.SessionID)
			a.mu.Unlock()
			switch {
			case t == nil || hidden:
				return nil, fmt.Errorf("%s is not connected to its agent; open its window and choose Reconnect Now, then send again", at.Name)
			case t.ID() == s.ID():
				return nil, fmt.Errorf("%s is this chat, which cannot ask itself", at.Name)
			}
			out = append(out, acp.ContentBlock{Type: "text", Text: "The user mentioned the chat " + strconv.Quote(a.chatTitle(t)) + " (session " + strconv.Quote(t.ID()) + ", workspace " + strconv.Quote(a.chatWorkspaceName(t.Cwd())) + "), another chat with its own agent and work. When the message wants something from that chat, ask it with " + chatAskAction + " of the bundled applet dinah.chat, giving this session ID and a question that is complete in itself, since that chat has not seen this conversation. Use applet_discover for the schema and applet_invoke once to ask. " + product.Name + " delivers completion automatically, as steering when supported while busy or as a new prompt otherwise. Continue independent work or end your turn; do not poll or sleep. A busy recipient receives steering when its provider and model support it; otherwise the message waits until its turn ends. Selection alone does not ask anything."})
		case "draft":
			if !notificationDraftID.MatchString(at.DraftID) {
				return nil, fmt.Errorf("%s is not a document open in the Editor", at.Name)
			}
			out = append(out, acp.ContentBlock{Type: "text", Text: draftMention(a.draftDocRef(&DocRef{DraftID: at.DraftID, Title: at.Name}))})
		case "link":
			if at.URI == "" {
				continue
			}
			out = append(out, acp.ContentBlock{Type: "resource_link", URI: at.URI, Name: at.Name, MimeType: at.MimeType})
		case "file":
			data, err := base64.StdEncoding.DecodeString(at.Data)
			if err != nil {
				return nil, fmt.Errorf("%s could not be read: %v", at.Name, err)
			}
			mime := at.MimeType
			switch {
			case strings.HasPrefix(mime, "image/") && caps.Image:
				picture := at.Data
				if m := providerModules[provider]; m.Picture != nil {
					var note string
					picture, note = m.Picture(at.Name, mime, data, at.Data)
					if note != "" {
						out = append(out, acp.ContentBlock{Type: "text", Text: note})
					}
				}
				out = append(out, acp.ContentBlock{Type: "image", Data: picture, MimeType: mime})
				continue
			case caps.EmbeddedContext && len(data) <= maxEmbeddedText && isText(mime, at.Name, data):
				p, err := a.saveAttachment(s, at.Name, data)
				if err != nil {
					return nil, err
				}
				out = append(out, acp.ContentBlock{Type: "resource", Resource: &acp.EmbeddedResource{URI: fileURI(p), MimeType: textMime(mime), Text: string(data)}})
				continue
			}
			p, err := a.saveAttachment(s, at.Name, data)
			if err != nil {
				return nil, err
			}
			out = append(out, acp.ContentBlock{Type: "resource_link", URI: fileURI(p), Name: at.Name, MimeType: mime})
		}
	}
	return out, nil
}

// claudePicture is a picture as a Claude chat is given it, in base64: with
// its longer side reduced to 2000 px when it is longer (fitPicture), or as
// it was. A picture over the size that cannot be rewritten, which is a WebP
// one, is sent as it was, with a note for the agent that says so; a picture
// that cannot be read is sent as it was, for the agent to report.
func claudePicture(name, mime string, data []byte, encoded string) (string, string) {
	fitted, changed, w, h, err := fitPicture(mime, data, claudeLongestSide)
	switch {
	case errors.Is(err, errCannotRewrite):
		logf("attach", "warn", "the picture %s is %dx%d px and %s cannot write %s, so it goes to Claude unchanged", name, w, h, product.Name, mime)
		return encoded, fmt.Sprintf("The picture %s is %d by %d px. %s reduces pictures over %d px on a side for Claude, but it cannot write %s, so this one is sent at its own size.", name, w, h, product.Name, claudeLongestSide, mime)
	case err != nil:
		logf("attach", "warn", "the picture %s could not be read to fit it for Claude: %v", name, err)
		return encoded, ""
	case changed:
		return base64.StdEncoding.EncodeToString(fitted), ""
	}
	return encoded, ""
}

// promptCaps is what the agent of a chat's provider says its prompts take,
// in its answer to initialize. Whether a picture goes as an image block
// rests on its image field (https://agentclientprotocol.com/protocol/
// initialization#prompt-capabilities); the protocol has no way for an agent
// to say this per model.
func (a *App) promptCaps(s *Session) acp.PromptCapabilities {
	a.mu.Lock()
	defer a.mu.Unlock()
	_, init := a.providerInitLocked(s.Provider())
	return init.AgentCapabilities.PromptCapabilities
}

// ChatImageInput says whether the chat's model can read pictures: "yes", "no"
// or "unknown", taken only from what is documented. Ollama lists "vision" among
// a model's capabilities (/api/show). The other agents say it once for the whole
// agent in their answer to initialize (promptCapabilities.image, which is false
// when absent). Devin's support is per model, in an extension that is not
// documented, so its chats stay unknown, as does any chat whose provider has not
// answered. The page warns only for "no".
func (a *App) ChatImageInput(id string) string {
	s := a.session(id)
	if s == nil {
		return "unknown"
	}
	provider := normalizedProvider(s.Provider())
	source := providerRegistry[provider].ImageInput
	if source != "adapter" && source != "initialize" {
		return "unknown"
	}
	a.mu.Lock()
	conn, init := a.providerInitLocked(provider)
	a.mu.Unlock()
	if conn == nil {
		return "unknown"
	}
	if source == "adapter" {
		var res ollamaagent.ImageInputResult
		if err := a.callWithin(conn, 15*time.Second, ollamaagent.ImageInputMethod, acp.CancelParams{SessionID: id}, &res); err != nil || res.ImageInput == "" {
			return "unknown"
		}
		return res.ImageInput
	}
	if init.AgentCapabilities.PromptCapabilities.Image {
		return "yes"
	}
	return "no"
}

// resourceAttachment is what something in a workspace comes down to when it
// is attached: a file in a local folder is linked where it is, for the agent
// to read.
func (a *App) resourceAttachment(ref ResourceRef, name string) (Attachment, error) {
	source, err := a.resourceService().source(ref)
	if err != nil {
		return Attachment{}, err
	}
	if source.Kind != "local-folder" {
		return Attachment{}, errors.New("only files in a local folder can be attached")
	}
	f, err := openWorkspaceResource(source, ref.Path)
	if err != nil {
		return Attachment{}, err
	}
	f.Close()
	rel, err := workspaceRelativePath(ref.Path)
	if err != nil {
		return Attachment{}, err
	}
	p := filepath.Join(source.Path, rel)
	return Attachment{Kind: "link", Name: name, URI: fileURI(p), MimeType: mime.TypeByExtension(strings.ToLower(filepath.Ext(p)))}, nil
}

var unsafeName = regexp.MustCompile(`[^\w.\- ()]+`)

// saveAttachment writes a pasted or dropped file into the session's
// workspace, under attachments/<date> in the profile's workspace folder
// (.dinah for Dinah), and returns its path.
func (a *App) saveAttachment(s *Session, name string, data []byte) (string, error) {
	name = strings.TrimSpace(unsafeName.ReplaceAllString(filepath.Base(name), "_"))
	if name == "" || name == "." {
		name = "attachment"
	}
	dir := filepath.Join(s.Cwd(), product.WorkspaceFolder, "attachments", time.Now().Format("2006-01-02"))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	p := filepath.Join(dir, name)
	ext := filepath.Ext(name)
	for i := 2; ; i++ {
		if _, err := os.Stat(p); os.IsNotExist(err) {
			break
		}
		p = filepath.Join(dir, fmt.Sprintf("%s (%d)%s", strings.TrimSuffix(name, ext), i, ext))
	}
	return p, os.WriteFile(p, data, 0o644)
}

func isText(mime, name string, data []byte) bool {
	if strings.HasPrefix(mime, "text/") || mime == "application/json" || mime == "message/rfc822" {
		return utf8.Valid(data)
	}
	switch strings.ToLower(filepath.Ext(name)) {
	case ".md", ".txt", ".csv", ".json", ".xml", ".yaml", ".yml", ".eml", ".log", ".properties", ".sql":
		return utf8.Valid(data)
	}
	return false
}

func textMime(m string) string {
	if m == "" {
		return "text/plain"
	}
	return m
}

// fileURI is a file:// URI for an absolute path: file:///C:/x/y.md on
// Windows, file:///x/y.md elsewhere.
func fileURI(p string) string {
	s := filepath.ToSlash(p)
	if !strings.HasPrefix(s, "/") {
		s = "/" + s
	}
	u := url.URL{Scheme: "file", Path: s}
	return u.String()
}
