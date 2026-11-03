package bus

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"studio/internal/agents"
	"studio/internal/fileaccess"
	"studio/internal/layout"
	"studio/internal/workspaces"
)

func (b *Bus) fileAccessPolicy() (*fileaccess.Service, error) {
	b.accessMu.Lock()
	defer b.accessMu.Unlock()
	if b.access != nil {
		return b.access, nil
	}
	resolve := b.accessDefaults
	if resolve == nil {
		resolve = fileaccess.CurrentDefaults
	}
	defaults, err := resolve()
	if err != nil {
		return nil, err
	}
	store, err := fileaccess.Open(filepath.Join(b.dir, "file-access.db"))
	if err != nil {
		return nil, err
	}
	b.access = &fileaccess.Service{Store: store, Defaults: defaults}
	return b.access, nil
}

func (b *Bus) attachmentScope(ctx context.Context, session Session, source string) (fileaccess.Scope, error) {
	service, err := b.fileAccessPolicy()
	if err != nil {
		return fileaccess.Scope{}, err
	}
	// Serialize registry snapshots and their publication so an older caller
	// cannot replace a newer caller's complete root set.
	b.accessMu.Lock()
	defer b.accessMu.Unlock()
	records, err := workspaces.Load(b.dir)
	if err != nil && !errors.Is(err, workspaces.ErrNoRegistry) {
		return fileaccess.Scope{}, err
	}
	// The bus also knows workspaces used without the desktop registry.
	known, err := b.Workspaces(ctx)
	if err != nil {
		return fileaccess.Scope{}, err
	}
	for _, workspace := range known {
		found := false
		for _, record := range records {
			if layout.SamePath(record.ExecutionPath, workspace.Path) {
				found = true
				break
			}
		}
		if !found {
			records = append(records, workspaces.Record{ID: workspace.Key, ExecutionPath: workspace.Path})
		}
	}
	if err := service.SetWorkspaceRoots(fileaccess.RegisteredWorkspaceRoots(records)); err != nil {
		return fileaccess.Scope{}, err
	}
	scope := fileaccess.WorkspaceScope(layout.Layout{State: b.dir}, records, session.WorkspacePath)
	scope.Chat, scope.ChatTitle, scope.Source = session.ID, session.Title, source
	scope.AuditAlways = true
	return scope, nil
}

func attachmentFailure(service *fileaccess.Service, scope fileaccess.Scope, path string, failure error) error {
	_, err := service.Store.Append(fileaccess.Record{Chat: scope.Chat, ChatTitle: scope.ChatTitle, Workspace: scope.Workspace, Path: path, Outcome: "error", Source: scope.Source, Reason: "The attachment could not be read safely"})
	if err != nil {
		return fmt.Errorf("could not commit attachment audit: %w", err)
	}
	return failure
}

func (b *Bus) readPolicyAttachment(ctx context.Context, session Session, path, source string, ownerRoot string, read func(*os.File) (string, error)) (string, error) {
	service, err := b.fileAccessPolicy()
	if err != nil {
		return "", err
	}
	scope, err := b.attachmentScope(ctx, session, source)
	if err != nil {
		return "", err
	}
	if ownerRoot != "" {
		resolved, err := fileaccess.Resolve(path)
		if err == nil && layout.Inside(ownerRoot, path) {
			root, err := fileaccess.Resolve(ownerRoot)
			if err == nil && layout.Inside(root, resolved) {
				scope.SharedLinkRoots = []string{root}
			}
		}
	}
	check, err := service.Check(path, scope)
	if err != nil {
		return "", err
	}
	if len(check.MatchedRuleIDs) > 0 {
		_, auditErr := service.Store.Append(fileaccess.Record{Chat: scope.Chat, ChatTitle: scope.ChatTitle, Workspace: scope.Workspace, Path: check.ResolvedPath, Outcome: "denied", RuleID: check.MatchedRuleIDs[0], Reason: check.Error, Source: source, UnverifiedRuleIDs: check.UnverifiedRuleIDs})
		if auditErr != nil {
			return "", fmt.Errorf("could not commit refused attachment audit: %w", auditErr)
		}
		return "", errors.New("this attachment path is refused: " + check.Error)
	}
	file, err := agents.OpenCatalogFile(path)
	if err != nil {
		return "", attachmentFailure(service, scope, path, err)
	}
	defer file.Close()
	return service.ReadOpened(ctx, path, scope, file, read)
}

func (b *Bus) hashAttachmentWithPolicy(ctx context.Context, session Session, path string) (string, int64, error) {
	var hash string
	var size int64
	_, err := b.readPolicyAttachment(ctx, session, path, "bus.attach", "", func(file *os.File) (string, error) {
		sum := sha256.New()
		var err error
		size, err = io.Copy(sum, file)
		if err != nil {
			return "", err
		}
		hash = hex.EncodeToString(sum.Sum(nil))
		return "", nil
	})
	return hash, size, err
}

func (b *Bus) readAttachmentWithPolicy(ctx context.Context, session Session, attachment Attachment, senderIDs ...string) ([]byte, error) {
	read := func(file *os.File) (string, error) {
		data, err := io.ReadAll(io.LimitReader(file, MaxAttachmentRead+1))
		if err != nil {
			return "", err
		}
		if len(data) > MaxAttachmentRead {
			return "", fmt.Errorf("%s exceeds the %d bytes this tool returns", attachment.Rel, MaxAttachmentRead)
		}
		return string(data), nil
	}
	if !attachment.Stored {
		ownerRoot := ""
		if len(senderIDs) == 1 {
			if sender, err := b.Session(ctx, senderIDs[0]); err == nil {
				ownerRoot = sender.WorkspacePath
			}
		}
		text, err := b.readPolicyAttachment(ctx, session, attachment.Path, "read_attachment", ownerRoot, read)
		return []byte(text), err
	}
	// Person-owned stored copies contain supplied bytes, not an agent-chosen
	// live pathname. Keep that catalogue authority confined to the copy store.
	service, err := b.fileAccessPolicy()
	if err != nil {
		return nil, err
	}
	scope, err := b.attachmentScope(ctx, session, "read_attachment")
	if err != nil {
		return nil, err
	}
	if !layout.Inside(b.attachmentDir(), attachment.Path) {
		return nil, attachmentFailure(service, scope, attachment.Path, errors.New("the attachment copy is outside its catalogue"))
	}
	file, err := agents.OpenCatalogFile(attachment.Path)
	if err != nil {
		return nil, attachmentFailure(service, scope, attachment.Path, err)
	}
	defer file.Close()
	text, err := read(file)
	if err != nil {
		return nil, attachmentFailure(service, scope, attachment.Path, err)
	}
	_, err = service.Store.Append(fileaccess.Record{Chat: session.ID, ChatTitle: session.Title, Workspace: scope.Workspace, Path: fmt.Sprintf("catalog:attachment:%s", attachment.Rel), Bytes: len(text), Outcome: "allowed", Source: "read_attachment", Reason: "Person-supplied attachment copy"})
	if err != nil {
		return nil, errors.New("could not commit attachment audit; no content returned")
	}
	return []byte(text), nil
}
