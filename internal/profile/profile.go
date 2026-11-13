// Package profile describes the product a build of this program is: its
// name, the folders and files it keeps, the names the operating system sees,
// its ports and its defaults. Every place that shows or uses one of these
// values reads it from the profile, so that a distribution can be built from
// the same source with a profile of its own.
//
// A build has one profile and cannot switch to another while it runs. This
// build has Dinah's. How another build supplies its own is decided with the
// public host interfaces.
//
// Some values stay out of the profile on purpose. Names written on the wire
// and read back, such as the dinah.* action IDs, the MCP servers dinah and
// dinah-bus, and markers such as "[Dinah chat: ...]", are Dinah's whatever
// the build. The applet helper's MCP server is the exception (AppletServer),
// because its name is written into a file every product on the computer
// shares. Files that are branding in their own right
// (the icon, the Windows resources, wails.json, the brand stylesheet, the
// logo and the theme data) stay files, and a distribution supplies its own
// copies.
package profile

import (
	"slices"
	"strconv"
)

// UpdaterInstalled is the updater that watches the installed window program,
// the one the command line's install puts in the home folder's bin folder,
// and offers to restart when a newer build replaces it.
const UpdaterInstalled = "installed"

// Profile is one product's values.
type Profile struct {
	// Name is the product's name as people see it, in the window, on its
	// pages and in what agents are told.
	Name string
	// ID is the product's lower-case identifier, used where a name has to be
	// a single plain word, such as the home folder's name when nothing better
	// is known.
	ID string
	// Company is the company that publishes the product.
	Company string

	// HomeFolder is the folder in the person's home folder where the
	// product keeps its state, unless the home variable (HomeVar) sets
	// another.
	HomeFolder string
	// WorkspaceFolder is the folder the product keeps inside a workspace,
	// for its runs and attachments.
	WorkspaceFolder string
	// LogFile is the log file's name, in the logs folder of the home folder.
	LogFile string
	// WorkspaceFileExtension is the extension of a workspace export's file,
	// with its dot (docs/design/workspace-export.md).
	WorkspaceFileExtension string
	// PackageManifest is where a content package keeps its manifest, as a
	// slash-separated path relative to the package's root, both in the
	// package's ZIP and in the folder it is installed in
	// (docs/design/packages.md).
	PackageManifest string
	// PackageTool is the program whose version a package's manifest may
	// require (its min_tool), as people know it in messages.
	PackageTool string
	// ReservedPrograms are the command-line programs whose credential and flow-answer
	// commands only the person may run, so an agent is refused them
	// (internal/agentpolicy). A build lists its own programs here.
	ReservedPrograms []string
	// EnvPrefix begins the environment variables a person sets, and the one
	// the product sets for its own agents (HomeVar and HostedVar).
	EnvPrefix string

	// CommandProgram is the command line's file name, without ".exe".
	CommandProgram string
	// WindowProgram is the window program's file name, without ".exe".
	WindowProgram string
	// InstanceLock is the name of the window's single-instance lock.
	InstanceLock string
	// WindowClass is the Windows class of the main window.
	WindowClass string
	// NotificationClass is the Windows class of the hidden window that owns
	// the product's notification icon.
	NotificationClass string
	// KeychainService is the service the product's credentials are kept
	// under in the system keychain.
	KeychainService string
	// ShellCookie is the cookie that signs the window's page in, and
	// BusCookie the one that signs a browser in to the bus's pages.
	ShellCookie string
	BusCookie   string

	// ShellPort is the loopback port the window's page is served on, and
	// BusPort the one the bus's pages are served on. Two products on one
	// computer must not share either.
	ShellPort int
	BusPort   int

	// AppletServer is the name of the MCP server through which an agent
	// uses the applets. Two products on one computer must not share it: an
	// agent that keeps its MCP servers in one file for the whole computer
	// has every product's entry in that file under this name, and its tools
	// are called mcp__<name>__<tool>.
	AppletServer string
	// ToolServers are further MCP servers the product gives the chats it
	// starts: programs of its own, installed beside the window program,
	// that offer agents tools the product does not offer as applets. The
	// window registers each for its agents by itself, so a new
	// installation needs no setup and the tools are the release the window
	// is (provider_devin.go). Dinah has none.
	ToolServers []ToolServer

	// DefaultProvider is the agent provider a new chat uses until the person
	// chooses another.
	DefaultProvider string
	// Providers are the agent providers the product offers, in the order
	// Settings lists them.
	Providers []string
	// DefaultTheme is the built-in theme the window uses until the person
	// chooses another.
	DefaultTheme string
	// Updater is how the product learns of a newer build: UpdaterInstalled,
	// or "" for none.
	Updater string
	// AgentsProposeDocumentChanges says that an agent may not change a
	// document through the document applications' actions: the actions
	// that change, save, make or close a document are left out of what
	// agents are offered, and an agent proposes its changes as suggestions
	// the person accepts or rejects (the Documents applet). The person's
	// own editing is the same either way.
	AgentsProposeDocumentChanges bool
}

// ToolServer is one MCP server of a product's own: the name agents know it
// by, the program that serves it, by its name without an extension, and the
// arguments the program is started with.
type ToolServer struct {
	Name    string
	Program string
	Args    []string
	// EarlierFolders are the folders, in the person's home folder, where an
	// earlier installation of the product put the program. An entry that
	// runs the program from one of them was written by that installation
	// and is brought to the program's place now. An entry that runs it
	// from anywhere else is a person's own.
	EarlierFolders []string
}

// dinah is Dinah's profile, the one this build uses.
var dinah = Profile{
	Name:    "Dinah",
	ID:      "dinah",
	Company: "Paul Parks",

	HomeFolder:      ".dinah",
	WorkspaceFolder: ".dinah",
	LogFile:         "dinah.log",
	EnvPrefix:       "DINAH_",

	ReservedPrograms: []string{"dinah"},

	WorkspaceFileExtension: ".dinah-workspace",
	PackageManifest:        ".dinah/setup-manifest.json",
	PackageTool:            "dinah",

	CommandProgram:    "dinah",
	WindowProgram:     "dinah-desktop",
	InstanceLock:      "dinah-desktop-4b1f0c6e",
	WindowClass:       "DinahWindow",
	NotificationClass: "DinahHostNotifications",
	KeychainService:   "dinah",
	ShellCookie:       "dinah_shell",
	BusCookie:         "dinah_bus",

	ShellPort: 47616,
	BusPort:   47615,

	AppletServer: "dinah-applets",

	DefaultProvider: "devin",
	Providers:       []string{"devin", "codex", "claude", "ollama"},
	DefaultTheme:    "clear-chime",
	Updater:         UpdaterInstalled,
}

// LegacyProvider is the provider of a chat saved before chats had providers,
// whatever the product's default provider is now. It never changes, since the
// chats it names are already saved.
const LegacyProvider = "devin"

// ProviderOrLegacy is a chat's provider: the one it has, or the legacy provider
// when it has none.
func ProviderOrLegacy(id string) string {
	if id == "" {
		return LegacyProvider
	}
	return id
}

// Current returns the profile this build uses: Dinah's, or the one another
// product's build injected (injected.go). The result is a copy, so a caller
// cannot change the profile for anybody else.
func Current() Profile {
	p := current
	p.Providers = slices.Clone(current.Providers)
	p.ReservedPrograms = slices.Clone(current.ReservedPrograms)
	return p
}

// Source returns the profile of the product this source is, whatever
// product the build is. Programs built from this source that must agree with
// each other on one name, whichever product each is, take it from here:
// where they keep the locks that arbitrate their saves of one file
// (internal/savelock), as HostedVars does for the variable they all honour.
func Source() Profile {
	p := dinah
	p.Providers = slices.Clone(dinah.Providers)
	p.ReservedPrograms = slices.Clone(dinah.ReservedPrograms)
	return p
}

// HomeVar is the environment variable that sets the home folder in place of
// HomeFolder in the person's home folder, such as DINAH_HOME.
func (p Profile) HomeVar() string { return p.EnvPrefix + "HOME" }

// ShellEntryVar is the environment variable that makes the window program
// start hidden and write its signed-in page address to the file the variable
// gives, for a browser test to drive the page, such as DINAH_SHELL_ENTRY_FILE.
func (p Profile) ShellEntryVar() string { return p.EnvPrefix + "SHELL_ENTRY_FILE" }

// ProvidersVar is the environment variable that, for development, replaces
// the agent providers the profile offers with a comma-separated list, such
// as DINAH_PROVIDERS=devin,codex (injected.go, withProviderOverride).
func (p Profile) ProvidersVar() string { return p.EnvPrefix + "PROVIDERS" }

// HostedVar is the environment variable the window sets for the agents it
// runs, such as DINAH_HOSTED.
func (p Profile) HostedVar() string { return p.EnvPrefix + "HOSTED" }

// HostedVars are the variables that say an agent runs in the window of a
// product built from this source: the product's own, and the one of the
// product the source is, when that is another. A person may have several
// such products, each with its command line program in the settings their
// agents load. A window sets every one of these for its agents, and each
// program stands down when it sees any of them, so that only the window's
// own bus server speaks for the chat. Before 10 November 2026 a product set
// and read its own alone, so in a second product's window the first one's
// bus server offered its tools without knowing the chat, and agents chose
// them and failed.
func (p Profile) HostedVars() []string {
	vars := []string{p.HostedVar()}
	if base := dinah.HostedVar(); base != vars[0] {
		vars = append(vars, base)
	}
	return vars
}

// ShellAddr is the loopback address the window's page is served on.
func (p Profile) ShellAddr() string { return "127.0.0.1:" + strconv.Itoa(p.ShellPort) }

// BusAddr is the loopback address the bus's pages are served on.
func (p Profile) BusAddr() string { return "127.0.0.1:" + strconv.Itoa(p.BusPort) }

// Offers says whether the product offers the agent provider id.
func (p Profile) Offers(id string) bool { return slices.Contains(p.Providers, id) }
