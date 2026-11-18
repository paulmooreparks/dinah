package main

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"studio/internal/acp"
	"studio/internal/profile"
	"studio/internal/programs"
)

// echoToolsVar makes the test's own program stand in for a product's tool
// program: it prints its arguments as a line of JSON, answers each line of
// its input with a line that contains it, and ends with status 3 when its
// input ends.
const echoToolsVar = "DINAH_TEST_ECHO_TOOLS"

func TestMain(m *testing.M) {
	if os.Getenv(echoToolsVar) == "1" {
		os.Exit(echoTools(os.Stdin, os.Stdout))
	}
	os.Exit(m.Run())
}

func echoTools(in io.Reader, out io.Writer) int {
	first, _ := json.Marshal(map[string]any{"args": os.Args[1:]})
	fmt.Fprintf(out, "%s\n", first)
	lines := bufio.NewScanner(in)
	for lines.Scan() {
		reply, _ := json.Marshal(map[string]string{"echo": lines.Text()})
		fmt.Fprintf(out, "%s\n", reply)
	}
	return 3
}

// renamedProduct is a product that gave its command program the name its
// tool program had: the command program is example-cli, the tool program
// example-tools, and agents' settings written earlier start example-cli
// with "serve". The command program's own serve starts a web server.
func renamedProduct() profile.Profile {
	return profile.Profile{
		Name: "Example Product", ID: "example", Company: "Example Company",
		HomeFolder: ".example", WorkspaceFolder: ".example", LogFile: "example.log", EnvPrefix: "EXAMPLE_",
		ReservedPrograms:       []string{"example-cli"},
		WorkspaceFileExtension: ".example-workspace", PackageManifest: ".example/setup-manifest.json", PackageTool: "example-cli",
		CommandProgram: "example-cli", EarlierCommandPrograms: []string{"ex"}, WindowProgram: "example-desktop", InstanceLock: "example-desktop-1",
		WindowClass: "ExampleWindow", NotificationClass: "ExampleHostNotifications", KeychainService: "example",
		ShellCookie: "example_shell", BusCookie: "example_bus", ShellPort: 47916, BusPort: 47915,
		AppletServer: "example-applets",
		ToolServers:  []profile.ToolServer{{Name: "example-cli", Program: "example-tools", Args: []string{"serve"}, EarlierPrograms: []string{"example-cli"}}},
		ProgramCommands: []profile.ProgramCommand{
			{Name: "serve", Program: "example-tools", Summary: "serve the agent tools"},
		},
		DefaultProvider: "devin", Providers: []string{"devin"}, DefaultTheme: "example-theme", Updater: profile.UpdaterInstalled,
	}
}

// buildRenamedCommandProgram builds this command program as renamedProduct
// into dir, with the test's own program beside it as the tool program.
func buildRenamedCommandProgram(t *testing.T, dir string) string {
	t.Helper()
	gobin, err := exec.LookPath("go")
	if err != nil {
		t.Skip("the go program is not on the path")
	}
	data, err := json.Marshal(renamedProduct())
	if err != nil {
		t.Fatal(err)
	}
	cli := filepath.Join(dir, programs.FileName("example-cli"))
	build := exec.Command(gobin, "build", "-o", cli, "-ldflags", "-X studio/internal/profile.injected="+base64.StdEncoding.EncodeToString(data), ".")
	acp.HideWindow(build)
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("building the command program: %v\n%s", err, out)
	}
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	test, err := os.ReadFile(self)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, programs.FileName("example-tools")), test, 0o755); err != nil {
		t.Fatal(err)
	}
	return cli
}

// An agent's entry written before the rename starts the command program
// with "serve". The command program hands it to the tool program, ahead of
// its own serve, and the tool program's input and output are the agent's
// pipes, unchanged, in both directions: what the agent writes reaches the
// tool program, what the tool program writes reaches the agent, and its
// status is the command program's. The command program's own serve would
// print the address of a web server first; the first line here is the tool
// program's. The command program is started as an agent starts an MCP
// server, with pipes and no console window.
func TestAStaleServeEntryReachesTheToolProgramThroughTheCommandProgram(t *testing.T) {
	dir := t.TempDir()
	cli := buildRenamedCommandProgram(t, dir)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, cli, "serve", "--no-browser")
	cmd.Env = append(os.Environ(), echoToolsVar+"=1", "EXAMPLE_HOME="+t.TempDir())
	acp.HideWindow(cmd)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	var stderr strings.Builder
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	lines := bufio.NewReader(stdout)
	read := func() string {
		line, err := lines.ReadString('\n')
		if err != nil {
			t.Fatalf("reading the tool program's output: %v (so far %q; stderr %q)", err, line, stderr.String())
		}
		return strings.TrimSpace(line)
	}
	if first := read(); first != `{"args":["serve","--no-browser"]}` {
		t.Fatalf("the first line is not the tool program's: %q (stderr %q)", first, stderr.String())
	}
	request := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`
	if _, err := io.WriteString(stdin, request+"\n"); err != nil {
		t.Fatal(err)
	}
	want, _ := json.Marshal(map[string]string{"echo": request})
	if got := read(); got != string(want) {
		t.Fatalf("the reply is %q, want %q", got, want)
	}
	stdin.Close()
	err = cmd.Wait()
	var ended *exec.ExitError
	if !errors.As(err, &ended) || ended.ExitCode() != 3 {
		t.Fatalf("the command program ended with %v, want the tool program's status 3 (stderr %q)", err, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("the command program wrote to its error output: %q", stderr.String())
	}
}

// A command program that a product renamed says what it is, so that the
// window can tell it from the tool program that had its name.
func TestTheCommandProgramSaysWhatItIs(t *testing.T) {
	dir := t.TempDir()
	cli := buildRenamedCommandProgram(t, dir)
	cmd := exec.Command(cli, programs.IdentityCommand)
	cmd.Env = append(os.Environ(), "EXAMPLE_HOME="+t.TempDir())
	acp.HideWindow(cmd)
	out, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	id, err := programs.ParseIdentity(string(out))
	if err != nil {
		t.Fatalf("%v: %q", err, out)
	}
	if id.Product != "example" || id.Program != "example-cli" || id.Version == "" {
		t.Fatalf("identity %+v", id)
	}
}

// Dinah's own profile hands no command on, so its serve is its own.
func TestDinahHandsNoCommandOn(t *testing.T) {
	if profile.Injected() != "" {
		t.Skip("this test runs on Dinah's own build")
	}
	for _, name := range commands {
		if program, ok := forwarded(name); ok {
			t.Fatalf("%s is handed to %s", name, program)
		}
	}
}
