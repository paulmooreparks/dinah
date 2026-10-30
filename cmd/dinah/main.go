// Command dinah is Dinah's command line: the message bus's tools for the
// agents, the launchers for terminal agents, and the page server.
package main

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"studio/internal/bus"
	"studio/internal/hooks"
	"studio/internal/mcpserver"
	"studio/internal/profile"
	"studio/internal/server"
	"studio/internal/web"
)

// product is the profile this build was made with (internal/profile), which
// gives the command's name, the product's name and the home folder.
var product = profile.Current()

// usage is the command's help. The command lines are written for a command
// name as long as "dinah"; another profile's longer name moves the
// descriptions to the right of where the columns line up.
var usage = strings.NewReplacer(
	"{cmd}", product.CommandProgram,
	"{name}", product.Name,
	"{window}", product.WindowProgram,
	"{homevar}", product.HomeVar(),
	"{home}", product.HomeFolder,
).Replace(`{cmd} is {name}'s message bus.

  {cmd} install        copy this binary to the one place Claude Code runs it
                       from (use "go run ./cmd/dinah install")
  {cmd} setup          print what to add to Claude Code to join the bus
  {cmd} serve          serve {name} and open it in the browser ({window}
                       is the {name} window, which serves it itself)
  {cmd} open           sign a browser in to a page that is already served
  {cmd} send TO TEXT   send a message as paul
  {cmd} claude [ARGS]  start Claude Code here, live on the bus: it wakes
                       when a message arrives (ARGS go to claude)
  {cmd} codex [ARGS]   start Codex here on the bus: it reads its messages
                       when it next takes a turn (ARGS go to codex)
  {cmd} mcp            the MCP server Claude Code starts for each session
  {cmd} hook           {name}'s SessionStart and SessionEnd hook
  {cmd} completion powershell
                       print a script that completes {cmd}'s commands, for
                       a PowerShell profile to run through Invoke-Expression
  {cmd} ext            install, update, list or remove signed extensions
  {cmd} workspace list
                       list the workspaces, with their IDs and main folders
  {cmd} workspace export WORKSPACE [--out FILE]
                       write a workspace's structure for Import Workspace…
                       in {name} on another computer
  {cmd} package install ZIP [--personal] [--workspace DIR] [--dry-run]
                       install or update a package of skills, for you in
                       every workspace or in one workspace
  {cmd} package status [--workspace DIR]
                       list the packages installed for you and in a
                       workspace
  {cmd} package remove [--personal [--name NAME]] [--workspace DIR]
                       remove an installed package, backing up its files
  {cmd} version        print the version

{homevar} sets the folder that holds the bus (default: ~/{home}).
`)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	var err error
	switch os.Args[1] {
	case "mcp":
		err = runMCP(ctx, os.Args[2:])
	case "steering-hook":
		err = withBus(func(b *bus.Bus) error { return hooks.Steering(ctx, b, os.Stdin, os.Stdout) })
	case "hook":
		err = withBus(func(b *bus.Bus) error { return hooks.Run(ctx, b, os.Stdin, os.Stdout) })
	case "serve":
		err = serve(ctx, os.Args[2:])
	case "open":
		err = open(ctx, os.Args[2:])
	case "send":
		err = send(ctx, os.Args[2:])
	case "claude":
		err = launchClaude(os.Args[2:])
	case "codex":
		err = launchCodex(ctx, os.Args[2:])
	case "ext":
		err = extensionCommand(ctx, os.Args[2:])
	case "workspace":
		err = workspaceCommand(os.Args[2:], os.Stdout)
	case "package":
		err = packageCommand(os.Args[2:], os.Stdout)
	case "install":
		err = install()
	case "setup":
		err = printHookSetup()
	case "completion":
		err = completion(os.Args[2:])
	case "version":
		fmt.Println(product.CommandProgram, mcpserver.Version)
	case "help", "-h", "--help":
		fmt.Print(usage)
	default:
		fmt.Fprintf(os.Stderr, "%s: unknown command %q\n\n%s", product.CommandProgram, os.Args[1], usage)
		os.Exit(2)
	}
	if err != nil && !errors.Is(err, context.Canceled) {
		fmt.Fprintln(os.Stderr, product.CommandProgram+":", err)
		os.Exit(1)
	}
}

// commands are the first arguments completion offers.
var commands = []string{"ext", "workspace", "package", "install", "setup", "serve", "open", "send", "claude", "codex", "mcp", "hook", "completion", "version", "help"}

// completion prints a PowerShell script that registers a completer for
// dinah's commands, through PowerShell's documented Register-ArgumentCompleter.
func completion(args []string) error {
	if len(args) != 1 || args[0] != "powershell" {
		return errors.New("usage: " + product.CommandProgram + " completion powershell")
	}
	quoted := make([]string, len(commands))
	for i, c := range commands {
		quoted[i] = "'" + c + "'"
	}
	fmt.Printf(`Register-ArgumentCompleter -Native -CommandName %[1]s, %[1]s.exe -ScriptBlock {
    param($wordToComplete, $commandAst, $cursorPosition)
    if ($commandAst.CommandElements.Count -gt 2 -or ($commandAst.CommandElements.Count -eq 2 -and $wordToComplete -eq '')) { return }
    @(%[2]s) | Where-Object { $_ -like "$wordToComplete*" } | ForEach-Object {
        [System.Management.Automation.CompletionResult]::new($_, $_, 'ParameterValue', $_)
    }
}
`, product.CommandProgram, strings.Join(quoted, ", "))
	return nil
}

func withBus(f func(*bus.Bus) error) error {
	b, err := bus.Open()
	if err != nil {
		return err
	}
	defer b.Close()
	return f(b)
}

func serve(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	addr := fs.String("addr", server.DefaultAddr, "the loopback address and port to serve on")
	noBrowser := fs.Bool("no-browser", false, "do not open the browser")
	fs.Parse(args)
	srv, err := server.Start(ctx, *addr)
	if err != nil {
		return err
	}
	defer srv.Close()
	u, err := srv.SigninURL(ctx)
	if err != nil {
		return err
	}
	fmt.Printf("%s is at http://%s/\n", product.Name, *addr)
	if *noBrowser {
		fmt.Printf("Sign a browser in within two minutes at %s\n", u)
	} else if err := browse(u); err != nil {
		fmt.Printf("Open %s within two minutes to sign in.\n", u)
	}
	return srv.Serve(ctx)
}

func open(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("open", flag.ExitOnError)
	addr := fs.String("addr", server.DefaultAddr, "the address the page is served on")
	fs.Parse(args)
	return withBus(func(b *bus.Bus) error {
		u, err := web.SigninURL(ctx, b, *addr)
		if err != nil {
			return err
		}
		if err := browse(u); err != nil {
			fmt.Printf("Open %s within two minutes to sign in.\n", u)
		}
		return nil
	})
}

// browse opens a URL in the default browser through each platform's
// documented command for opening a document.
func browse(u string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", "", u)
	case "darwin":
		cmd = exec.Command("open", u)
	default:
		cmd = exec.Command("xdg-open", u)
	}
	return cmd.Start()
}

func send(ctx context.Context, args []string) error {
	if len(args) < 2 {
		return errors.New("usage: " + product.CommandProgram + " send TO TEXT")
	}
	return withBus(func(b *bus.Bus) error {
		m, err := b.Send(ctx, bus.Sender{}, args[0], strings.Join(args[1:], " "), nil, 0)
		if err != nil {
			return err
		}
		fmt.Printf("Sent as message %d to %s.\n", m.ID, m.ToPerson)
		return nil
	})
}

// install copies the running binary to bin in Dinah's home folder and, when
// run from Dinah's source folder, builds the window program there too.
func install() error {
	home, err := bus.Home()
	if err != nil {
		return err
	}
	dir := filepath.Join(home, "bin")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	self, err := os.Executable()
	if err != nil {
		return err
	}
	dst := filepath.Join(dir, exeName(product.CommandProgram))
	if err := place(self, dst); err != nil {
		return err
	}
	fmt.Printf("Installed %s. Sessions started from now on use it.\n", dst)

	if _, err := os.Stat(filepath.Join("cmd", "dinah-desktop", "main.go")); err != nil {
		fmt.Printf("Run this from %[1]s's source folder to install the %[1]s window too.\n", product.Name)
		return nil
	}
	tmp, err := os.MkdirTemp("", product.WindowProgram+"-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	built := filepath.Join(tmp, exeName(product.WindowProgram))
	args := []string{"build", "-tags", "desktop,production", "-o", built}
	if runtime.GOOS == "windows" {
		args = append(args, "-ldflags", "-H windowsgui")
	}
	cmd := exec.Command("go", append(args, "./cmd/dinah-desktop")...)
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("building the %s window: %w", product.Name, err)
	}
	dst = filepath.Join(dir, exeName(product.WindowProgram))
	if err := place(built, dst); err != nil {
		return err
	}
	fmt.Printf("Installed %s. Close the %s window and start it again to use it.\n", dst, product.Name)
	return nil
}

func exeName(name string) string {
	if runtime.GOOS == "windows" {
		return name + ".exe"
	}
	return name
}

// place copies the program src to dst. Something may be running the program
// at dst, and Windows refuses to overwrite a running program but allows it
// to be renamed, so the old one is moved aside first. Copies set aside
// earlier are removed once nothing runs them.
func place(src, dst string) error {
	old, _ := filepath.Glob(dst + ".old-*")
	for _, o := range old {
		os.Remove(o) // fails while something still runs it; it goes next time
	}
	// The copy is written beside dst before anything is moved, so src is still
	// readable when it is dst itself (installing from the installed program),
	// and a failed copy leaves the installed program alone. It is renamed into
	// place, so dst is never a half-written program: the window watches it to
	// offer Restart to Update.
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	tmp := dst + ".new"
	out, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		in.Close()
		return err
	}
	_, err = io.Copy(out, in)
	in.Close() // closed before the rename below: Windows will not rename a program that is open
	if err != nil {
		out.Close()
		os.Remove(tmp)
		return err
	}
	if err := out.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	aside := ""
	if _, err := os.Stat(dst); err == nil {
		aside = fmt.Sprintf("%s.old-%d", dst, time.Now().UnixNano())
		if err := renameFile(dst, aside); err != nil {
			os.Remove(tmp)
			return err
		}
	}
	if err := renameFile(tmp, dst); err != nil {
		os.Remove(tmp)
		if aside != "" {
			if back := renameFile(aside, dst); back != nil {
				return fmt.Errorf("%w (and the previous program could not be put back: %v)", err, back)
			}
		}
		return err
	}
	return nil
}

// renameFile is os.Rename; a test replaces it to fail a step of place.
var renameFile = os.Rename

// runMCP serves MCP for one session. Claude Code starts it with no flags and
// its hooks identify the session; a Dinah launcher passes --session.
func runMCP(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("mcp", flag.ExitOnError)
	session := fs.String("session", "", "the session a "+product.Name+" launcher registered")
	fs.Parse(args)
	return withBus(func(b *bus.Bus) error {
		if *session == "" && os.Getenv(bus.HostedVar) != "" {
			return mcpserver.NewIdle(b).Run(ctx)
		}
		if *session == "" {
			return mcpserver.New(b).Run(ctx)
		}
		if _, err := b.Session(ctx, *session); err != nil {
			return err
		}
		return mcpserver.NewLaunched(b, *session).Run(ctx)
	})
}

// launchCodex starts an interactive Codex session in the current folder with
// Dinah's MCP server attached for this launch only, through Codex's
// documented -c overrides of mcp_servers keys, so the person's Codex config
// is not changed. The launcher registers the session on the bus and passes
// its ID to the server; Codex documents no way to wake an idle session, so
// the agent reads its messages when it next takes a turn.
func launchCodex(ctx context.Context, args []string) error {
	codex, err := exec.LookPath("codex")
	if err != nil {
		return errors.New("codex is not on the PATH")
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	id := "dinah-" + rand.Text()
	return withBus(func(b *bus.Bus) error {
		s, err := b.RegisterSession(ctx, id, cwd, "Codex")
		if err != nil {
			return err
		}
		defer b.EndSession(context.Background(), id)
		fmt.Printf("On the bus as %s.\n", s.Address())
		// Each -c value is one argument, so no shell quoting applies; the
		// values are TOML, as Codex documents.
		cmd := exec.Command(codex, append([]string{
			"-c", "mcp_servers.dinah.command=" + tomlString(exe),
			"-c", `mcp_servers.dinah.args=["mcp", "--session", ` + tomlString(id) + `]`,
		}, args...)...)
		cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
		return cmd.Run()
	})
}

// tomlString quotes s as a TOML basic string.
func tomlString(s string) string {
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(s) + `"`
}

// launchClaude starts an interactive Claude Code session in the current
// folder with Dinah loaded as a development channel, so the bus can wake
// it. Custom channels need the development flag during the channels research
// preview. The opening prompt fires the UserPromptSubmit hook, which tells
// the bus which session this is; until then the doorbell cannot find it.
func launchClaude(args []string) error {
	claude, err := exec.LookPath("claude")
	if err != nil {
		return errors.New("claude is not on the PATH")
	}
	if len(args) == 0 {
		args = []string{"You are live on the " + product.Name + " bus. Call the dinah whoami tool and say in one line who you are. Then wait; messages will wake you."}
	}
	// The development-channels flag takes every argument after it as a
	// channel entry, so it goes last.
	cmd := exec.Command(claude, append(args, "--dangerously-load-development-channels", "server:dinah")...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	return cmd.Run()
}

// HookConfig returns the hooks block for Claude Code's settings that joins
// every session to the bus.
func HookConfig(exe string) map[string]any {
	command := func() []any {
		return []any{map[string]any{"hooks": []any{map[string]any{"type": "command", "command": exe, "args": []any{"hook"}}}}}
	}
	deliver := func(event string) []any {
		input := map[string]any{"session_id": "${session_id}", "event": event, "cwd": "${cwd}"}
		// The prompt carries the marker that ties a chat in Dinah's window
		// to its agent's own session, for steering it mid-turn.
		if event == "UserPromptSubmit" {
			input["prompt"] = "${prompt}"
		}
		return []any{map[string]any{"hooks": []any{map[string]any{
			"type": "mcp_tool", "server": "dinah", "tool": "hook_deliver", "input": input,
		}}}}
	}
	return map[string]any{
		"SessionStart":     command(),
		"SessionEnd":       command(),
		"UserPromptSubmit": deliver("UserPromptSubmit"),
		"PostToolUse":      deliver("PostToolUse"),
		"Stop":             deliver("Stop"),
	}
}

// printHookSetup prints what to add to Claude Code to join every session to
// the bus, for "setup".
func printHookSetup() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	settings := map[string]any{
		"hooks":       HookConfig(exe),
		"permissions": map[string]any{"allow": []any{"mcp__dinah", "mcp__dinah-bus"}},
	}
	out, _ := json.MarshalIndent(settings, "", "  ")
	fmt.Printf(`To join every Claude Code session to the bus:

1. Register the MCP server for all your projects:

   claude mcp add --scope user dinah -- "%s" mcp

2. Merge these hooks and this permission into ~/.claude/settings.json,
   alongside whatever is already there:

%s

3. Run "%s serve" to see the bus's page.

Sessions that are already running join when they are restarted.
`, exe, out, product.CommandProgram)
	return nil
}
