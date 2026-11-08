package acp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"testing"
	"time"
)

type closePipeHandler struct{}

func (closePipeHandler) Notify(string, json.RawMessage) {}
func (closePipeHandler) Request(context.Context, string, json.RawMessage) (any, *Error) {
	return nil, nil
}

type childPipeHandler struct{ pid chan int }

func (h childPipeHandler) Notify(_ string, raw json.RawMessage) {
	var value struct {
		PID int `json:"pid"`
	}
	if json.Unmarshal(raw, &value) == nil {
		h.pid <- value.PID
	}
}
func (childPipeHandler) Request(context.Context, string, json.RawMessage) (any, *Error) {
	return nil, nil
}

func TestCloseWithDescendantHoldingStdout(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	h := childPipeHandler{pid: make(chan int, 1)}
	c, err := Start(exe, []string{"-test.run=^TestOutputPipeHelper$"}, t.TempDir(), h, Options{Env: append(os.Environ(), "DINAH_PIPE_HELPER=parent"), TraceHeadersOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	select {
	case pid := <-h.pid:
		child, err := os.FindProcess(pid)
		if err != nil {
			t.Fatal(err)
		}
		defer child.Kill()
	case <-time.After(5 * time.Second):
		t.Fatal("helper did not start its child")
	}
	c.Close()
	select {
	case <-c.Done():
	case <-time.After(7 * time.Second):
		t.Fatal("provider close waited for the descendant's output handle")
	}
}

func TestOutputPipeHelper(t *testing.T) {
	mode := os.Getenv("DINAH_PIPE_HELPER")
	if mode == "" {
		return
	}
	if mode == "parent" {
		exe, _ := os.Executable()
		child := exec.Command(exe, "-test.run=^TestOutputPipeHelper$")
		child.Env = append(os.Environ(), "DINAH_PIPE_HELPER=child")
		child.Stdout = os.Stdout
		child.Stderr = os.Stderr
		hideWindow(child)
		if err := child.Start(); err != nil {
			os.Exit(2)
		}
		fmt.Printf("{\"method\":\"child\",\"params\":{\"pid\":%d}}\n", child.Process.Pid)
	}
	time.Sleep(15 * time.Second)
	os.Exit(0)
}

func TestCloseReleasesReaderEvenWhenWriterRemainsOpen(t *testing.T) {
	reader, writer := io.Pipe()
	defer writer.Close()
	defer reader.Close()
	input, output := io.Pipe()
	defer input.Close()
	c := Connect(reader, output, closePipeHandler{}, Options{})
	c.Close()
	select {
	case <-c.Done():
	case <-time.After(time.Second):
		t.Fatal("Close left the reader blocked on a still-open output pipe")
	}
}
