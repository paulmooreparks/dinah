//go:build windows

package terminals

import (
	"context"
	"encoding/binary"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// WCT can wait while inspecting another process. Keep all native queries in
// a disposable helper, after the original test deadline has already failed.
func nodeWaitDiagnostic(shellPID int, since time.Time, target ...uint32) string {
	if shellPID <= 0 {
		return "wait diagnostic unavailable: no live shell PID"
	}
	executable, err := os.Executable()
	if err != nil {
		return "wait diagnostic unavailable: " + err.Error()
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, executable, "-test.run=^TestNodeWaitDiagnosticHelper$")
	cmd.WaitDelay = 500 * time.Millisecond
	cmd.Env = append(os.Environ(), "TERMINAL_WAIT_SHELL="+strconv.Itoa(shellPID), "TERMINAL_WAIT_SINCE="+strconv.FormatInt(since.UnixNano(), 10))
	if len(target) != 0 {
		cmd.Env = append(cmd.Env, "TERMINAL_WAIT_PID="+strconv.FormatUint(uint64(target[0]), 10))
	}
	output, err := cmd.CombinedOutput()
	return fmt.Sprintf("%s (helper error %v, deadline %v)", output, err, ctx.Err())
}

// wctNode follows WAITCHAIN_NODE_INFO from wct.h. The union's largest member
// contains WCHAR[128], LARGE_INTEGER and BOOL, rounded to eight-byte alignment.
// Object names are deliberately omitted from diagnostic output.
type wctNode struct {
	objectType   uint32
	objectStatus uint32
	object       [34]uint64
}

func TestNodeWaitDiagnosticHelper(t *testing.T) {
	shell, err := strconv.ParseUint(os.Getenv("TERMINAL_WAIT_SHELL"), 10, 32)
	if err != nil || shell == 0 {
		return
	}
	if os.Getenv("TERMINAL_WAIT_STALL") == "1" {
		time.Sleep(time.Minute)
		return
	}
	since, err := strconv.ParseInt(os.Getenv("TERMINAL_WAIT_SINCE"), 10, 64)
	if err != nil {
		fmt.Println("invalid diagnostic creation time:", err)
		return
	}
	if err := enableWaitDiagnosticPrivilege(); err != nil {
		fmt.Println("debug privilege request:", err)
	}
	dll := windows.NewLazySystemDLL("advapi32.dll")
	open := dll.NewProc("OpenThreadWaitChainSession")
	query := dll.NewProc("GetThreadWaitChain")
	closeSession := dll.NewProc("CloseThreadWaitChainSession")
	for _, proc := range []*windows.LazyProc{open, query, closeSession} {
		if err := proc.Find(); err != nil {
			fmt.Println("WCT unavailable:", err)
			return
		}
	}
	session, _, callErr := open.Call(0, 0)
	if session == 0 {
		fmt.Println("WCT session unavailable:", callErr)
		return
	}
	defer closeSession.Call(session)
	nodes := waitDiagnosticNodes(uint32(shell), time.Unix(0, since))
	if target, err := strconv.ParseUint(os.Getenv("TERMINAL_WAIT_PID"), 10, 32); err == nil && target != 0 {
		for pid := range nodes {
			if pid != uint32(target) {
				delete(nodes, pid)
			}
		}
	}
	if len(nodes) == 0 {
		fmt.Println("no fixture-owned Node process found")
		return
	}
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPTHREAD, 0)
	if err != nil {
		fmt.Println("thread snapshot:", err)
		return
	}
	defer windows.CloseHandle(snapshot)
	entry := windows.ThreadEntry32{Size: uint32(unsafe.Sizeof(windows.ThreadEntry32{}))}
	count := 0
	for err = windows.Thread32First(snapshot, &entry); err == nil; err = windows.Thread32Next(snapshot, &entry) {
		if !nodes[entry.OwnerProcessID] {
			continue
		}
		if count == 64 {
			fmt.Println("thread diagnostic truncated at 64 threads")
			break
		}
		count++
		var chain [16]wctNode
		length := uint32(len(chain))
		var cycle uint32
		// Follow waits and inspect critical sections in other processes.
		// WCT_OUT_OF_PROC_FLAG | WCT_OUT_OF_PROC_CS_FLAG = 1 | 4.
		ok, _, queryErr := query.Call(session, 0, 5, uintptr(entry.ThreadID), uintptr(unsafe.Pointer(&length)), uintptr(unsafe.Pointer(&chain[0])), uintptr(unsafe.Pointer(&cycle)))
		if ok != 0 {
			queryErr = nil // LastError has no defined meaning on success.
		}
		fmt.Printf("pid=%d thread=%d success=%t error=%v cycle=%t nodes=%d", entry.OwnerProcessID, entry.ThreadID, ok != 0, queryErr, cycle != 0, length)
		if ok != 0 || queryErr == windows.ERROR_MORE_DATA {
			if length > uint32(len(chain)) {
				length = uint32(len(chain))
			}
			for _, node := range chain[:length] {
				fmt.Printf(" -> type=%d status=%d", node.objectType, node.objectStatus)
				if node.objectType == 8 { // WctThreadType
					data := unsafe.Slice((*byte)(unsafe.Pointer(&node.object[0])), 16)
					fmt.Printf(" pid=%d thread=%d wait=%d switches=%d", binary.LittleEndian.Uint32(data), binary.LittleEndian.Uint32(data[4:]), binary.LittleEndian.Uint32(data[8:]), binary.LittleEndian.Uint32(data[12:]))
				}
			}
		}
		fmt.Println()
	}
	if err != nil && err != windows.ERROR_NO_MORE_FILES {
		fmt.Println("thread enumeration:", err)
	}
}

func waitDiagnosticNodes(shell uint32, since time.Time) map[uint32]bool {
	found := make(map[uint32]bool)
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		fmt.Println("process snapshot:", err)
		return found
	}
	defer windows.CloseHandle(snapshot)
	entry := windows.ProcessEntry32{Size: uint32(unsafe.Sizeof(windows.ProcessEntry32{}))}
	for err = windows.Process32First(snapshot, &entry); err == nil; err = windows.Process32Next(snapshot, &entry) {
		if entry.ParentProcessID != shell || !strings.EqualFold(windows.UTF16ToString(entry.ExeFile[:]), "node.exe") {
			continue
		}
		process, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, entry.ProcessID)
		if err != nil {
			fmt.Printf("candidate pid=%d could not be inspected: %v\n", entry.ProcessID, err)
			continue
		}
		var created, exited, kernel, user windows.Filetime
		err = windows.GetProcessTimes(process, &created, &exited, &kernel, &user)
		windows.CloseHandle(process)
		if err == nil && !time.Unix(0, created.Nanoseconds()).Before(since) {
			found[entry.ProcessID] = true
		}
	}
	return found
}

func enableWaitDiagnosticPrivilege() error {
	var token windows.Token
	if err := windows.OpenProcessToken(windows.CurrentProcess(), windows.TOKEN_ADJUST_PRIVILEGES|windows.TOKEN_QUERY, &token); err != nil {
		return err
	}
	defer token.Close()
	name, _ := windows.UTF16PtrFromString("SeDebugPrivilege")
	state := windows.Tokenprivileges{PrivilegeCount: 1}
	if err := windows.LookupPrivilegeValue(nil, name, &state.Privileges[0].Luid); err != nil {
		return err
	}
	state.Privileges[0].Attributes = windows.SE_PRIVILEGE_ENABLED
	// AdjustTokenPrivileges can succeed while returning NOT_ALL_ASSIGNED.
	proc := windows.NewLazySystemDLL("advapi32.dll").NewProc("AdjustTokenPrivileges")
	ok, _, err := proc.Call(uintptr(token), 0, uintptr(unsafe.Pointer(&state)), 0, 0, 0)
	if ok == 0 || err != windows.ERROR_SUCCESS {
		return err
	}
	return nil
}

func TestNodeWaitDiagnosticFindsOnlyFixtureChild(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Skipf("node is not installed: %v", err)
	}
	since := time.Now()
	child := exec.Command("node", "-e", "setInterval(()=>{},1000)")
	if err := child.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = child.Process.Kill(); _ = child.Wait() }()
	result := nodeWaitDiagnostic(os.Getpid(), since, uint32(child.Process.Pid))
	t.Log(result)
	if !strings.Contains(result, fmt.Sprintf("pid=%d thread=", child.Process.Pid)) {
		t.Fatalf("fixture child was not inspected: %s", result)
	}
	decodedChild := false
	for _, node := range strings.Split(result, " -> ")[1:] {
		if strings.HasPrefix(node, "type=8 ") && strings.Contains(node, fmt.Sprintf(" pid=%d thread=", child.Process.Pid)) {
			decodedChild = true
		}
	}
	if !decodedChild {
		t.Fatalf("WCT did not return a decoded thread belonging to the fixture child: %s", result)
	}
	unrelated := nodeWaitDiagnostic(child.Process.Pid, since)
	if !strings.Contains(unrelated, "no fixture-owned Node process found") {
		t.Fatalf("a child of another parent was inspected: %s", unrelated)
	}
	unrelated = nodeWaitDiagnostic(os.Getpid(), since, uint32(os.Getpid()))
	if !strings.Contains(unrelated, "no fixture-owned Node process found") {
		t.Fatalf("a different target PID was inspected: %s", unrelated)
	}
}

func TestNodeWaitDiagnosticBoundsStalledHelper(t *testing.T) {
	t.Setenv("TERMINAL_WAIT_STALL", "1")
	start := time.Now()
	result := nodeWaitDiagnostic(os.Getpid(), start)
	if !strings.Contains(result, "deadline context deadline exceeded") {
		t.Fatalf("stalled helper did not report its deadline: %s", result)
	}
	if elapsed := time.Since(start); elapsed > 6*time.Second {
		t.Fatalf("stalled helper took %s: %s", elapsed, result)
	}
}
