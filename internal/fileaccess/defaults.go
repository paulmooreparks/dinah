package fileaccess

import (
	"path"
	"strings"
)

// DefaultRules resolves only published conventional locations and host-provided
// overrides. It does not inspect any credential or browser-profile contents.
func defaultRules(platform, home, roaming, local string, env map[string]string) []Rule {
	join := func(base, tail string) string {
		if base == "" {
			return ""
		}
		p := path.Join(strings.ReplaceAll(base, "\\", "/"), tail)
		if platform == "windows" {
			return strings.ReplaceAll(p, "/", "\\")
		}
		return p
	}
	// XDG requires absolute values. Relative overrides must be ignored.
	envCopy := map[string]string{}
	for k, v := range env {
		envCopy[k] = v
	}
	env = envCopy
	isAbs := func(p string) bool {
		if platform == "windows" {
			return len(p) >= 3 && p[1] == ':' && (p[2] == '/' || p[2] == '\\') || strings.HasPrefix(p, "\\\\")
		}
		return strings.HasPrefix(p, "/")
	}
	for _, key := range []string{"XDG_CONFIG_HOME", "XDG_DATA_HOME"} {
		if !isAbs(env[key]) {
			delete(env, key)
		}
	}
	config := env["XDG_CONFIG_HOME"]
	if config == "" {
		config = join(home, ".config")
	}
	data := env["XDG_DATA_HOME"]
	if data == "" {
		data = join(home, ".local/share")
	}
	rules := []Rule{}
	add := func(id, pattern, reason string, paths ...string) {
		seen := map[string]bool{}
		clean := []string{}
		for _, p := range paths {
			if p != "" && !seen[p] {
				clean = append(clean, p)
				seen[p] = true
			}
		}
		rules = append(rules, Rule{ID: id, Kind: "default", Pattern: pattern, Reason: reason, Enabled: true, ResolvedPaths: clean, Examples: map[string]string{platform: strings.Join(clean, "; ")}, Actions: []string{"disable"}})
	}
	add("ssh", "{home}/.ssh and SSH_AUTH_SOCK", "SSH private keys and authentication-agent endpoints", join(home, ".ssh"), env["SSH_AUTH_SOCK"])
	add("aws", "{home}/.aws and AWS credential overrides", "Cloud account credentials", join(home, ".aws"), env["AWS_SHARED_CREDENTIALS_FILE"], env["AWS_CONFIG_FILE"])
	add("azure", "{home}/.azure or AZURE_CONFIG_DIR", "Cloud account sign-in data", join(home, ".azure"), env["AZURE_CONFIG_DIR"])
	gcloud := join(config, "gcloud")
	if platform == "windows" {
		gcloud = join(roaming, "gcloud")
	}
	add("gcloud", "Cloud SDK configuration and credential override", "Cloud account sign-in data", gcloud, env["CLOUDSDK_CONFIG"], env["GOOGLE_APPLICATION_CREDENTIALS"])
	kube := []string{join(home, ".kube")}
	sep := ":"
	if platform == "windows" {
		sep = ";"
	}
	kube = append(kube, strings.Split(env["KUBECONFIG"], sep)...)
	add("kubernetes", "{home}/.kube and KUBECONFIG entries", "Cluster credentials", kube...)
	add("npm", "{home}/.npmrc or NPM_CONFIG_USERCONFIG", "Package registry authentication", join(home, ".npmrc"), env["NPM_CONFIG_USERCONFIG"])
	cargo := env["CARGO_HOME"]
	if cargo == "" {
		cargo = join(home, ".cargo")
	}
	packagePaths := []string{join(home, ".pypirc"), join(cargo, "credentials.toml"), join(cargo, "credentials"), join(home, ".m2/settings.xml"), join(home, ".nuget/NuGet/NuGet.Config"), join(config, "NuGet/NuGet.Config")}
	if platform == "windows" {
		packagePaths = append(packagePaths, join(roaming, "NuGet/NuGet.Config"))
	}
	add("package-credentials", "Package publishing and repository credential files", "Package registry passwords and tokens", packagePaths...)
	add("git-credentials", "Git credential-store files", "Stored Git passwords and tokens", join(home, ".git-credentials"), join(config, "git/credentials"))
	add("docker", "{home}/.docker or DOCKER_CONFIG", "Container-registry authentication", join(home, ".docker"), env["DOCKER_CONFIG"])
	add("gnupg", "{home}/.gnupg or GNUPGHOME", "Private signing and encryption keys", join(home, ".gnupg"), join(roaming, "gnupg"), env["GNUPGHOME"])
	add("password-store", "{home}/.password-store or PASSWORD_STORE_DIR", "Password-store data", join(home, ".password-store"), env["PASSWORD_STORE_DIR"])
	switch platform {
	case "windows":
		add("browser-profiles", "Browser profile directories", "Browser passwords, cookies and sign-in sessions", join(local, "Google/Chrome/User Data"), join(local, "Chromium/User Data"), join(local, "Microsoft/Edge/User Data"), join(roaming, "Mozilla/Firefox"), join(local, "BraveSoftware/Brave-Browser/User Data"), join(roaming, "Opera Software"), join(local, "Vivaldi/User Data"))
		add("password-managers", "Password manager application data", "Password manager data and settings", join(roaming, "1Password"), join(local, "1Password"), join(roaming, "Bitwarden"), join(roaming, "Bitwarden CLI"), join(roaming, "KeePassXC"), join(local, "KeePassXC"))
		add("credential-stores", "Windows credential storage directories", "Operating-system credential data", join(roaming, "Microsoft/Credentials"), join(local, "Microsoft/Credentials"), join(roaming, "Microsoft/Vault"), join(local, "Microsoft/Vault"), join(roaming, "Microsoft/Protect"))
	case "darwin":
		add("browser-profiles", "Browser profile directories", "Browser passwords, cookies and sign-in sessions", join(home, "Library/Application Support/Google/Chrome"), join(home, "Library/Application Support/Chromium"), join(home, "Library/Application Support/Microsoft Edge"), join(home, "Library/Application Support/Firefox"), join(home, "Library/Safari"), join(home, "Library/Application Support/BraveSoftware/Brave-Browser"), join(home, "Library/Application Support/com.operasoftware.Opera"), join(home, "Library/Application Support/Vivaldi"))
		add("password-managers", "Password manager application data", "Password manager data and settings", join(home, "Library/Group Containers/2BUA8C4S2C.com.1password"), join(home, "Library/Application Support/Bitwarden"), join(home, "Library/Containers/com.bitwarden.desktop"), join(home, "Library/Application Support/KeePassXC"))
		add("credential-stores", "{home}/Library/Keychains", "Operating-system credential data", join(home, "Library/Keychains"))
	default:
		chromeConfig := config
		if env["CHROME_CONFIG_HOME"] != "" {
			chromeConfig = env["CHROME_CONFIG_HOME"]
		}
		add("browser-profiles", "Browser profile directories", "Browser passwords, cookies and sign-in sessions", join(chromeConfig, "google-chrome"), join(chromeConfig, "chromium"), join(config, "microsoft-edge"), join(home, ".mozilla/firefox"), env["CHROME_USER_DATA_DIR"], join(config, "BraveSoftware/Brave-Browser"), join(config, "opera"), join(config, "vivaldi"), join(home, "snap/firefox/common/.mozilla/firefox"), join(home, "snap/chromium/common/chromium"), join(home, ".var/app/org.mozilla.firefox"), join(home, ".var/app/org.chromium.Chromium"), join(home, ".var/app/com.google.Chrome"), join(home, ".var/app/com.microsoft.Edge"), join(home, ".var/app/com.brave.Browser"), join(home, ".var/app/com.opera.Opera"), join(home, ".var/app/com.vivaldi.Vivaldi"))
		add("password-managers", "Password manager application data", "Password manager data and settings", join(config, "1Password"), join(home, "snap/1password/current/.config/1Password"), join(config, "Bitwarden"), join(config, "Bitwarden CLI"), join(config, "keepassxc"), join(home, ".var/app/com.bitwarden.desktop"), join(home, ".var/app/org.keepassxc.KeePassXC"))
		add("credential-stores", "XDG data keyring directories", "Desktop credential data", join(data, "keyrings"), join(data, "kwalletd"))
	}
	add("network-credentials", "netrc and PostgreSQL password files", "Network service passwords", join(home, ".netrc"), join(home, "_netrc"), join(home, ".pgpass"), env["PGPASSFILE"], join(roaming, "postgresql/pgpass.conf"))
	add("password-overrides", "Password manager storage overrides", "Password manager data and settings", env["BITWARDEN_APPDATA_DIR"], env["KPXC_CONFIG"], env["KPXC_CONFIG_LOCAL"])
	gh := join(config, "gh")
	if platform == "windows" && env["XDG_CONFIG_HOME"] == "" {
		gh = join(roaming, "GitHub CLI")
	}
	add("github-cli", "GitHub CLI configuration or GH_CONFIG_DIR", "GitHub authentication tokens", gh, env["GH_CONFIG_DIR"])
	add("provider-credentials", "Agent account credentials and local model state", "Agent account tokens and private state", join(home, ".claude/.credentials.json"), join(home, ".codex/auth.json"), join(home, ".ollama"), join(env["CLAUDE_CONFIG_DIR"], ".credentials.json"), join(env["CODEX_HOME"], "auth.json"))
	add("terraform", "Terraform CLI credential configuration", "Infrastructure service credentials", join(home, ".terraform.d"), join(home, ".terraformrc"), join(roaming, "terraform.d"), join(roaming, "terraform.rc"), env["TF_CLI_CONFIG_FILE"])
	add("other-workspaces", "Other registered workspaces outside this workspace's roots", "Another workspace's data")
	return rules
}

func DefaultRules(platform, home, roaming, local string, env map[string]string) []Rule {
	rules := defaultRules(platform, home, roaming, local, env)
	for _, p := range []string{"windows", "darwin", "linux"} {
		h := "/home/person"
		r, l := "", ""
		if p == "windows" {
			h = "C:/Users/person"
			r = h + "/AppData/Roaming"
			l = h + "/AppData/Local"
		}
		if p == "darwin" {
			h = "/Users/person"
		}
		examples := defaultRules(p, h, r, l, nil)
		for i := range rules {
			for _, example := range examples {
				if example.ID == rules[i].ID {
					rules[i].Examples[p] = strings.Join(example.ResolvedPaths, "; ")
					break
				}
			}
		}
	}
	return rules
}
