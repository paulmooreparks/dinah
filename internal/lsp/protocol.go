package lsp

import "encoding/json"

// The method names this server answers and originates, spelled once each.
const (
	methodInitialize             = "initialize"
	methodInitialized            = "initialized"
	methodShutdown               = "shutdown"
	methodExit                   = "exit"
	methodDidOpen                = "textDocument/didOpen"
	methodDidChange              = "textDocument/didChange"
	methodDidClose               = "textDocument/didClose"
	methodDidSave                = "textDocument/didSave"
	methodHover                  = "textDocument/hover"
	methodDocumentLink           = "textDocument/documentLink"
	methodDefinition             = "textDocument/definition"
	methodCompletion             = "textDocument/completion"
	methodInlayHint              = "textDocument/inlayHint"
	methodDidChangeConfiguration = "workspace/didChangeConfiguration"
	methodInlayHintRefresh       = "workspace/inlayHint/refresh"
	methodConfiguration          = "workspace/configuration"
	methodLogMessage             = "window/logMessage"
	methodShowMessage            = "window/showMessage"
	// methodAnnotations is the namespaced request a client makes to read the
	// structured annotation it draws its own decoration from, which no
	// standard member of an inlay hint can carry.
	methodAnnotations = "dinah/annotations"
	// methodAnnotationsChanged is the namespaced notification the poll loop
	// sends when an open document's model was recomputed into something
	// different from what it replaced.
	methodAnnotationsChanged = "dinah/annotationsChanged"
)

// The message types window/logMessage and window/showMessage declare.
const (
	messageTypeError   = 1
	messageTypeWarning = 2
	messageTypeInfo    = 3
)

// Position is a zero-based line and UTF-16 code-unit offset within it.
type Position struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

// Range is a half-open span of one document.
type Range struct {
	Start Position `json:"start"`
	End   Position `json:"end"`
}

// Location is a range inside a named file.
type Location struct {
	URI   string `json:"uri"`
	Range Range  `json:"range"`
}

// textDocumentIdentifier names one open document.
type textDocumentIdentifier struct {
	URI string `json:"uri"`
}

// textDocumentItem is a document the client has just opened.
type textDocumentItem struct {
	URI        string `json:"uri"`
	LanguageID string `json:"languageId"`
	Version    int    `json:"version"`
	Text       string `json:"text"`
}

// didOpenParams carries the whole text of a newly opened document.
type didOpenParams struct {
	TextDocument textDocumentItem `json:"textDocument"`
}

// didChangeParams carries the whole text of a changed document, this server
// declaring full synchronisation.
type didChangeParams struct {
	TextDocument   textDocumentIdentifier `json:"textDocument"`
	ContentChanges []struct {
		Text string `json:"text"`
	} `json:"contentChanges"`
}

// documentParams is the shape of every request that names a document alone.
type documentParams struct {
	TextDocument textDocumentIdentifier `json:"textDocument"`
}

// positionParams is the shape of every request that names a position in a
// document.
type positionParams struct {
	TextDocument textDocumentIdentifier `json:"textDocument"`
	Position     Position               `json:"position"`
}

// initializeParams is the subset of the client's own declaration this server
// reads. Everything else the client sends is left alone.
type initializeParams struct {
	RootURI      string `json:"rootUri"`
	Capabilities struct {
		Workspace struct {
			Configuration bool `json:"configuration"`
			InlayHint     struct {
				RefreshSupport bool `json:"refreshSupport"`
			} `json:"inlayHint"`
		} `json:"workspace"`
	} `json:"capabilities"`
	WorkspaceFolders []struct {
		URI string `json:"uri"`
	} `json:"workspaceFolders"`
	InitializationOptions json.RawMessage `json:"initializationOptions"`
}

// syncOptions is the textDocumentSync capability, declared in full so a
// reader of the initialize result sees which of the three members is set.
type syncOptions struct {
	OpenClose bool `json:"openClose"`
	Change    int  `json:"change"`
	Save      struct {
		IncludeText bool `json:"includeText"`
	} `json:"save"`
}

// resolveOption is the shape of a capability whose whole declaration is
// whether it resolves.
type resolveOption struct {
	ResolveProvider bool `json:"resolveProvider"`
}

// completionOptions declares the trigger characters and that no item of a
// completion list is resolved in a second round trip.
type completionOptions struct {
	TriggerCharacters []string `json:"triggerCharacters"`
	ResolveProvider   bool     `json:"resolveProvider"`
}

// capabilities is the whole of what this server declares. A capability absent
// from this struct is one the server does not offer, and diagnostics are the
// absence that matters: dinah-264 owns them, and nothing here publishes one.
type capabilities struct {
	TextDocumentSync     syncOptions       `json:"textDocumentSync"`
	HoverProvider        bool              `json:"hoverProvider"`
	DocumentLinkProvider resolveOption     `json:"documentLinkProvider"`
	DefinitionProvider   bool              `json:"definitionProvider"`
	CompletionProvider   completionOptions `json:"completionProvider"`
	InlayHintProvider    resolveOption     `json:"inlayHintProvider"`
}

// serverInfo names the server and the build answering.
type serverInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// initializeResult is what the client is handed back.
type initializeResult struct {
	Capabilities capabilities `json:"capabilities"`
	ServerInfo   serverInfo   `json:"serverInfo"`
}

// markupContent is the hover body, always markdown here.
type markupContent struct {
	Kind  string `json:"kind"`
	Value string `json:"value"`
}

// hoverResult is what a hover answers when it answers anything.
type hoverResult struct {
	Contents markupContent `json:"contents"`
	Range    *Range        `json:"range,omitempty"`
}

// documentLink is one clickable span.
type documentLink struct {
	Range  Range  `json:"range"`
	Target string `json:"target"`
}

// inlayHint is the standard inline label. The chip a VS Code client draws is
// not one of these; it is built from the dinah/annotations answer, because no
// member of this type carries a structured payload an extension can read.
type inlayHint struct {
	Position    Position `json:"position"`
	Label       string   `json:"label"`
	PaddingLeft bool     `json:"paddingLeft"`
}

// completionItem is one candidate.
type completionItem struct {
	Label      string `json:"label"`
	InsertText string `json:"insertText"`
	Detail     string `json:"detail,omitempty"`
	SortText   string `json:"sortText,omitempty"`
	FilterText string `json:"filterText,omitempty"`
}

// completionList is the answer to a completion request, carrying whether the
// list was cut short.
type completionList struct {
	IsIncomplete bool             `json:"isIncomplete"`
	Items        []completionItem `json:"items"`
}

// annotationTarget names what an annotation's reference resolved to, in the
// canonical tokens the machine surface already uses. It is never translated.
type annotationTarget struct {
	Kind string `json:"kind"`
	ID   string `json:"id"`
	Ref  string `json:"ref"`
	// Card is the identifier of the card an entity below one belongs to, and
	// null for an entity that belongs to no card. It is a pointer so the
	// absence travels as null rather than as an empty string, which a client
	// would have to know to read as absence.
	Card *string `json:"card"`
}

// wireAnnotation is one entry of the dinah/annotations answer. Label and
// tooltip are for a person and are rendered through the catalogues; target
// and fields are for the machine and are canonical tokens, so a client
// colours on a token rather than by parsing a sentence.
type wireAnnotation struct {
	Range   Range             `json:"range"`
	Label   string            `json:"label"`
	Tooltip string            `json:"tooltip"`
	Target  *annotationTarget `json:"target"`
	Fields  map[string]string `json:"fields"`
	Link    string            `json:"link,omitempty"`
}

// annotationsResult is the dinah/annotations answer.
type annotationsResult struct {
	Annotations []wireAnnotation `json:"annotations"`
}

// annotationsChangedParams names the documents whose models moved.
type annotationsChangedParams struct {
	URIs []string `json:"uris"`
}

// logMessageParams is a line for the client's own log.
type logMessageParams struct {
	Type    int    `json:"type"`
	Message string `json:"message"`
}

// configurationParams asks the client for a settings section.
type configurationParams struct {
	Items []configurationItem `json:"items"`
}

// configurationItem names one section of the client's settings.
type configurationItem struct {
	Section string `json:"section"`
}

// didChangeConfigurationParams carries the settings a client pushes.
type didChangeConfigurationParams struct {
	Settings json.RawMessage `json:"settings"`
}

// settings is the shape this server reads out of its own configuration
// section, whether pulled at startup or pushed afterwards.
type settings struct {
	AnnotateProse       *bool `json:"annotateProse"`
	PollIntervalSeconds *int  `json:"pollIntervalSeconds"`
}
