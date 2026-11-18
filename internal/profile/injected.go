package profile

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"regexp"
	"slices"
	"strings"
)

// A build of another product supplies its profile at link time, so that
// nothing in this repository names or describes that product
// (docs/design/product-definition.md). The build passes the profile as a
// JSON document, encoded in standard base64 so that it survives the linker's
// command line, in the string the linker sets here:
//
//	go build -ldflags "-X studio/internal/profile.injected=<base64 of the JSON>"
//
// Dinah's own builds set nothing, and Current() is Dinah's. A profile that
// is set must be whole and sound, or the program stops before it starts:
// an incomplete profile would mix one product's names with another's.
var injected string

// current is the profile this build uses: Dinah's, or the injected one.
var current = chosen()

// Injected is the encoded profile this build was given, or "" for Dinah's
// own build. The command line's install passes it on to the window it
// builds, so an installed product is built as the product it is.
func Injected() string { return injected }

func chosen() Profile {
	p := dinah
	if injected != "" {
		var err error
		p, err = parseInjected(injected)
		if err != nil {
			panic("the build's product profile is unusable: " + err.Error())
		}
	}
	return withProviderOverride(p, os.Getenv(p.ProvidersVar()))
}

// withProviderOverride applies the environment's choice of agent providers
// to a profile, for development: the variable ProvidersVar names, with
// commas, the providers the build offers in place of the profile's. The
// default provider stays when it is among them and is otherwise the first.
// An empty or malformed value leaves the profile as it is.
func withProviderOverride(p Profile, value string) Profile {
	var providers []string
	for _, id := range strings.Split(value, ",") {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if !providerID.MatchString(id) {
			return p
		}
		providers = append(providers, id)
	}
	if len(providers) == 0 {
		return p
	}
	p.Providers = providers
	if !slices.Contains(providers, p.DefaultProvider) {
		p.DefaultProvider = providers[0]
	}
	return p
}

// providerID is the form of an agent provider's identifier.
var providerID = regexp.MustCompile(`^[a-z][a-z0-9-]{0,63}$`)

// parseInjected reads a profile from its base64-encoded JSON and checks that
// every value a product must give is given.
func parseInjected(encoded string) (Profile, error) {
	data, err := base64.StdEncoding.DecodeString(strings.TrimSpace(encoded))
	if err != nil {
		return Profile{}, fmt.Errorf("the profile is not base64: %w", err)
	}
	var p Profile
	dec := json.NewDecoder(strings.NewReader(string(data)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&p); err != nil {
		return Profile{}, fmt.Errorf("the profile is not a profile document: %w", err)
	}
	if err := p.check(); err != nil {
		return Profile{}, err
	}
	return p, nil
}

// check says what an injected profile lacks or gets wrong, all of it at once.
func (p Profile) check() error {
	var missing []string
	need := func(name, value string) {
		if strings.TrimSpace(value) == "" {
			missing = append(missing, name)
		}
	}
	need("Name", p.Name)
	need("ID", p.ID)
	need("Company", p.Company)
	need("HomeFolder", p.HomeFolder)
	need("WorkspaceFolder", p.WorkspaceFolder)
	need("LogFile", p.LogFile)
	need("EnvPrefix", p.EnvPrefix)
	need("WorkspaceFileExtension", p.WorkspaceFileExtension)
	need("PackageManifest", p.PackageManifest)
	need("PackageTool", p.PackageTool)
	need("CommandProgram", p.CommandProgram)
	need("WindowProgram", p.WindowProgram)
	need("InstanceLock", p.InstanceLock)
	need("WindowClass", p.WindowClass)
	need("NotificationClass", p.NotificationClass)
	need("KeychainService", p.KeychainService)
	need("ShellCookie", p.ShellCookie)
	need("BusCookie", p.BusCookie)
	need("DefaultProvider", p.DefaultProvider)
	need("DefaultTheme", p.DefaultTheme)
	var problems []string
	if len(missing) > 0 {
		problems = append(problems, "these values are missing: "+strings.Join(missing, ", "))
	}
	if p.ShellPort <= 0 || p.ShellPort > 65535 || p.BusPort <= 0 || p.BusPort > 65535 {
		problems = append(problems, "ShellPort and BusPort must be ports")
	} else if p.ShellPort == p.BusPort {
		problems = append(problems, "ShellPort and BusPort must differ")
	}
	if !providerID.MatchString(p.AppletServer) {
		problems = append(problems, "AppletServer must be a lowercase letter followed by lowercase letters, digits or hyphens")
	}
	for _, s := range p.ToolServers {
		if !providerID.MatchString(s.Name) || s.Name == p.AppletServer || !providerID.MatchString(s.Program) {
			problems = append(problems, "each of ToolServers needs a Name and a Program of lowercase letters, digits and hyphens, and a Name that is not AppletServer")
			break
		}
	}
	for _, c := range p.ProgramCommands {
		if !providerID.MatchString(c.Name) || !providerID.MatchString(c.Program) || c.Program == p.CommandProgram || strings.TrimSpace(c.Summary) == "" || strings.ContainsAny(c.Summary, "\r\n") {
			problems = append(problems, "each of ProgramCommands needs a Name and a Program of lowercase letters, digits and hyphens, a Program that is not CommandProgram, and a Summary of one line")
			break
		}
	}
	problems = append(problems, p.earlierNameProblems()...)
	if len(p.Providers) == 0 {
		problems = append(problems, "Providers must list at least one agent provider")
	} else if p.DefaultProvider != "" && !slices.Contains(p.Providers, p.DefaultProvider) {
		problems = append(problems, "DefaultProvider must be one of Providers")
	}
	if len(p.ReservedPrograms) == 0 {
		problems = append(problems, "ReservedPrograms must list the product's own programs")
	}
	if p.Updater != "" && p.Updater != UpdaterInstalled {
		problems = append(problems, "Updater must be "+UpdaterInstalled+" or empty")
	}
	if len(problems) == 0 {
		return nil
	}
	return errors.New(strings.Join(problems, "; "))
}

// earlierNameProblems says what is wrong with the earlier names of the
// product's programs. An earlier name of the command program must not be a
// program the product has now, since the window removes the file of that
// name once nothing uses it. An earlier name of a tool server's program may
// be the command program's name now, provided the command program hands
// the server's first argument on to the server's program; otherwise an
// agent's entry that still runs the earlier name would start the command
// program's own command.
func (p Profile) earlierNameProblems() []string {
	var problems []string
	current := []string{p.CommandProgram, p.WindowProgram}
	for _, s := range p.ToolServers {
		current = append(current, s.Program)
	}
	for _, c := range p.ProgramCommands {
		current = append(current, c.Program)
	}
	for _, name := range p.EarlierCommandPrograms {
		if !providerID.MatchString(name) || slices.Contains(current, name) {
			problems = append(problems, "each of EarlierCommandPrograms must be lowercase letters, digits and hyphens, and not the name of a program the product has now")
			break
		}
	}
	for _, s := range p.ToolServers {
		for _, name := range s.EarlierPrograms {
			if !providerID.MatchString(name) || name == s.Program || name == p.WindowProgram {
				problems = append(problems, "each of a tool server's EarlierPrograms must be lowercase letters, digits and hyphens, and neither its Program nor WindowProgram")
				break
			}
			if name == p.CommandProgram && len(s.Args) > 0 && !p.handsOn(s.Args[0], s.Program) {
				problems = append(problems, "a tool server whose earlier program is CommandProgram needs a ProgramCommand that hands its first argument, "+s.Args[0]+", to "+s.Program)
				break
			}
		}
	}
	return problems
}

// handsOn reports whether the command program hands the command name to
// program.
func (p Profile) handsOn(name, program string) bool {
	return slices.ContainsFunc(p.ProgramCommands, func(c ProgramCommand) bool { return c.Name == name && c.Program == program })
}
