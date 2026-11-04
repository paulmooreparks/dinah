//go:build windows

package terminals

import (
	"context"
	"encoding/binary"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// File version resources can be read without executing the target image.
func executableFileMetadata(path string) string {
	executable, err := os.Executable()
	if err != nil {
		return "metadata helper unavailable: " + err.Error()
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, executable, "-test.run=^TestExecutableMetadataHelper$")
	command.WaitDelay = 500 * time.Millisecond
	command.Env = append(os.Environ(), "TERMINAL_METADATA_PATH="+path)
	output, err := command.CombinedOutput()
	return fmt.Sprintf("%s (helper error %v, context error %v)", output, err, ctx.Err())
}

func TestExecutableMetadataHelper(t *testing.T) {
	if path := os.Getenv("TERMINAL_METADATA_PATH"); path != "" {
		fmt.Println(readExecutableFileMetadata(path))
	}
}

func readExecutableFileMetadata(path string) string {
	size, err := windows.GetFileVersionInfoSize(path, nil)
	if err != nil || size == 0 {
		return fmt.Sprintf("version resource unavailable: size=%d error=%v", size, err)
	}
	data := make([]byte, size)
	if err := windows.GetFileVersionInfo(path, 0, size, unsafe.Pointer(&data[0])); err != nil {
		return "version resource read: " + err.Error()
	}
	var info *windows.VS_FIXEDFILEINFO
	var length uint32
	if err := windows.VerQueryValue(unsafe.Pointer(&data[0]), `\`, unsafe.Pointer(&info), &length); err != nil {
		return "version resource query: " + err.Error()
	}
	if info == nil || length < uint32(unsafe.Sizeof(windows.VS_FIXEDFILEINFO{})) || info.Signature != 0xfeef04bd {
		return "invalid fixed version resource"
	}
	result := fmt.Sprintf("fileVersion=%d.%d.%d.%d productVersion=%d.%d.%d.%d", info.FileVersionMS>>16, info.FileVersionMS&65535, info.FileVersionLS>>16, info.FileVersionLS&65535, info.ProductVersionMS>>16, info.ProductVersionMS&65535, info.ProductVersionLS>>16, info.ProductVersionLS&65535)
	runtime.KeepAlive(data)
	return result
}

// A disposable process bounds a native dump call even if DbgHelp blocks.
// This runs after the startup or readiness deadline has already expired.
func nodeDumpDiagnostic(shellPID int, since time.Time, target ...uint32) string {
	dir := os.Getenv("DINAH_TEST_DIAGNOSTICS_DIR")
	if dir == "" {
		return "dump not requested: DINAH_TEST_DIAGNOSTICS_DIR is unset"
	}
	if !filepath.IsAbs(dir) {
		return "dump unavailable: diagnostics directory is not absolute"
	}
	executable, err := os.Executable()
	if err != nil {
		return "dump helper unavailable: " + err.Error()
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, executable, "-test.run=^TestNodeDumpDiagnosticHelper$")
	command.WaitDelay = 500 * time.Millisecond
	command.Env = append(os.Environ(), "TERMINAL_DUMP_SHELL="+strconv.Itoa(shellPID), "TERMINAL_DUMP_SINCE="+strconv.FormatInt(since.UnixNano(), 10))
	if len(target) != 0 {
		command.Env = append(command.Env, "TERMINAL_DUMP_PID="+strconv.FormatUint(uint64(target[0]), 10))
	}
	output, err := command.CombinedOutput()
	return fmt.Sprintf("%s (helper error %v, context error %v)", output, err, ctx.Err())
}

func TestNodeDumpDiagnosticHelper(t *testing.T) {
	parent, err := strconv.ParseUint(os.Getenv("TERMINAL_DUMP_SHELL"), 10, 32)
	if err != nil || parent == 0 {
		return
	}
	if os.Getenv("TERMINAL_DUMP_STALL") == "1" {
		time.Sleep(time.Minute)
		return
	}
	since, err := strconv.ParseInt(os.Getenv("TERMINAL_DUMP_SINCE"), 10, 64)
	if err != nil {
		t.Fatal(err)
	}
	dir := os.Getenv("DINAH_TEST_DIAGNOSTICS_DIR")
	if !filepath.IsAbs(dir) {
		t.Fatal("dump directory must be absolute")
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	writeDump := windows.NewLazySystemDLL("dbghelp.dll").NewProc("MiniDumpWriteDump")
	if err := writeDump.Find(); err != nil {
		t.Fatal(err)
	}
	// Calls are serial in this helper, as required by DbgHelp's contract.
	nodes := waitDiagnosticNodes(uint32(parent), time.Unix(0, since))
	if len(nodes) == 0 {
		fmt.Println("no fixture-owned Node process found")
	}
	for pid := range nodes {
		if target, err := strconv.ParseUint(os.Getenv("TERMINAL_DUMP_PID"), 10, 32); err == nil && target != 0 && pid != uint32(target) {
			continue
		}
		process, err := windows.OpenProcess(windows.PROCESS_QUERY_INFORMATION|windows.PROCESS_VM_READ, false, pid)
		if err != nil {
			fmt.Printf("pid %d dump access: %v\n", pid, err)
			continue
		}
		file, err := os.CreateTemp(dir, fmt.Sprintf("node-startup-%d-*.dmp", pid))
		if err != nil {
			windows.CloseHandle(process)
			t.Fatal(err)
		}
		// MiniDumpNormal plus MiniDumpWithThreadInfo. No full-memory capture.
		ok, _, dumpErr := writeDump.Call(uintptr(process), uintptr(pid), file.Fd(), 0x1000, 0, 0, 0)
		closeErr := file.Close()
		windows.CloseHandle(process)
		if ok != 0 {
			dumpErr = nil
		}
		fmt.Printf("pid %d dump %q success %t error %v close error %v\n", pid, file.Name(), ok != 0, dumpErr, closeErr)
	}
}

func TestNodeDumpDiagnosticCapturesOwnedChild(t *testing.T) {
	node, err := exec.LookPath("node.exe")
	if err != nil {
		t.Skip("node.exe is not installed")
	}
	dir := t.TempDir()
	t.Setenv("DINAH_TEST_DIAGNOSTICS_DIR", dir)
	since := time.Now()
	child := exec.Command(node, "-e", "setInterval(()=>{},1000)")
	if err := child.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = child.Process.Kill(); _ = child.Wait() }()
	result := nodeDumpDiagnostic(os.Getpid(), since, uint32(child.Process.Pid))
	t.Log(result)
	files, err := filepath.Glob(filepath.Join(dir, fmt.Sprintf("node-startup-%d-*.dmp", child.Process.Pid)))
	if err != nil || len(files) != 1 {
		t.Fatalf("owned child dump missing: %v %v %s", files, err, result)
	}
	data, err := os.ReadFile(files[0])
	if err != nil || len(data) < 32 || string(data[:4]) != "MDMP" {
		t.Fatalf("invalid dump: length %d error %v", len(data), err)
	}
	streams, offset := binary.LittleEndian.Uint32(data[8:]), binary.LittleEndian.Uint32(data[12:])
	threadList := false
	for i := uint32(0); i < streams; i++ {
		entry := uint64(offset) + uint64(i)*12
		if entry+12 > uint64(len(data)) {
			t.Fatal("truncated dump directory")
		}
		if binary.LittleEndian.Uint32(data[entry:]) == 3 {
			threadList = true
		}
	}
	if !threadList {
		t.Fatal("dump has no thread list")
	}
	other := nodeDumpDiagnostic(child.Process.Pid, since)
	other += nodeDumpDiagnostic(os.Getpid(), since, uint32(os.Getpid()))
	remaining, _ := filepath.Glob(filepath.Join(dir, "*.dmp"))
	if len(remaining) != 1 {
		t.Fatalf("unrelated parent produced a dump: %s", other)
	}
}

func TestNodeDumpDiagnosticBoundsStalledHelper(t *testing.T) {
	t.Setenv("DINAH_TEST_DIAGNOSTICS_DIR", t.TempDir())
	t.Setenv("TERMINAL_DUMP_STALL", "1")
	started := time.Now()
	result := nodeDumpDiagnostic(os.Getpid(), started)
	if !strings.Contains(result, "context error context deadline exceeded") || time.Since(started) > 8*time.Second {
		t.Fatalf("dump helper bound was lost: %s", result)
	}
}

func TestExecutableMetadataDoesNotRequireStartup(t *testing.T) {
	root := os.Getenv("SystemRoot")
	metadata := executableFileMetadata(filepath.Join(root, "System32", "kernel32.dll"))
	if !strings.HasPrefix(metadata, "fileVersion=") {
		t.Fatal(metadata)
	}
	if metadata := executableFileMetadata(filepath.Join(t.TempDir(), "missing.exe")); !strings.Contains(metadata, "unavailable") {
		t.Fatal(metadata)
	}
}
