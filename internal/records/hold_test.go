package records

import (
	"bufio"
	"errors"
	"os"
	"os/exec"
	"testing"
)

func TestHoldInOneProcess(t *testing.T) {
	s := Store{Dir: t.TempDir()}
	if s.Held("a") {
		t.Fatal("a record nobody took is held")
	}
	release, err := s.Hold("a")
	if err != nil {
		t.Fatal(err)
	}
	if !s.Held("a") {
		t.Fatal("a held record is not reported held")
	}
	if _, err := s.Hold("a"); !errors.Is(err, ErrHeld) {
		t.Fatalf("a second hold was granted: %v", err)
	}
	if other, err := s.Hold("b"); err != nil {
		t.Fatalf("one record's hold blocked another: %v", err)
	} else {
		other()
	}
	if err := release(); err != nil {
		t.Fatal(err)
	}
	if s.Held("a") {
		t.Fatal("a released record is still held")
	}
	again, err := s.Hold("a")
	if err != nil {
		t.Fatalf("a released record could not be taken again: %v", err)
	}
	again()
	if ID("a.hold") != "" {
		t.Fatal("a hold file was taken for a record")
	}
}

// The helper process holds a record until its input closes.
func TestHoldHelper(t *testing.T) {
	dir := os.Getenv("DINAH_HOLD_HELPER")
	if dir == "" {
		t.Skip("run by TestHoldAcrossProcesses")
	}
	if _, err := (Store{Dir: dir}).Hold("a"); err != nil {
		os.Stdout.WriteString("error " + err.Error() + "\n")
		return
	}
	os.Stdout.WriteString("held\n")
	_, _ = bufio.NewReader(os.Stdin).ReadString('\n')
}

func TestHoldAcrossProcesses(t *testing.T) {
	s := Store{Dir: t.TempDir()}
	cmd := exec.Command(os.Args[0], "-test.run=^TestHoldHelper$")
	cmd.Env = append(os.Environ(), "DINAH_HOLD_HELPER="+s.Dir)
	stdin, _ := cmd.StdinPipe()
	stdout, _ := cmd.StdoutPipe()
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	line, _ := bufio.NewReader(stdout).ReadString('\n')
	if line != "held\n" {
		t.Fatalf("the helper did not hold the record: %q", line)
	}
	if !s.Held("a") {
		t.Fatal("another process's hold is not seen")
	}
	if _, err := s.Hold("a"); !errors.Is(err, ErrHeld) {
		t.Fatalf("a record another process holds was granted: %v", err)
	}
	// The helper ends without releasing; the system releases its lock.
	if err := cmd.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	_ = cmd.Wait()
	stdin.Close()
	release, err := s.Hold("a")
	if err != nil {
		t.Fatalf("the hold of an ended process was not released: %v", err)
	}
	release()
}
