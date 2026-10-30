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
// and read back, such as the dinah.* action IDs, the MCP servers dinah,
// dinah-bus and dinah-applets, and markers such as "[Dinah chat: ...]", are
// Dinah's whatever the build. Files that are branding in their own right
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

	DefaultProvider: "devin",
	Providers:       []string{"devin", "codex", "claude", "ollama"},
	DefaultTheme:    "mineral",
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

// Current returns the profile this build uses. The result is a copy, so a
// caller cannot change the profile for anybody else.
func Current() Profile {
	p := dinah
	p.Providers = slices.Clone(dinah.Providers)
	p.ReservedPrograms = slices.Clone(dinah.ReservedPrograms)
	return p
}

// HomeVar is the environment variable that sets the home folder in place of
// HomeFolder in the person's home folder, such as DINAH_HOME.
func (p Profile) HomeVar() string { return p.EnvPrefix + "HOME" }

// HostedVar is the environment variable the window sets for the agents it
// runs, such as DINAH_HOSTED.
func (p Profile) HostedVar() string { return p.EnvPrefix + "HOSTED" }

// ShellAddr is the loopback address the window's page is served on.
func (p Profile) ShellAddr() string { return "127.0.0.1:" + strconv.Itoa(p.ShellPort) }

// BusAddr is the loopback address the bus's pages are served on.
func (p Profile) BusAddr() string { return "127.0.0.1:" + strconv.Itoa(p.BusPort) }

// Offers says whether the product offers the agent provider id.
func (p Profile) Offers(id string) bool { return slices.Contains(p.Providers, id) }
