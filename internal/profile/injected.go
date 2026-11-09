package profile

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
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
	if injected == "" {
		return dinah
	}
	p, err := parseInjected(injected)
	if err != nil {
		panic("the build's product profile is unusable: " + err.Error())
	}
	return p
}

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
