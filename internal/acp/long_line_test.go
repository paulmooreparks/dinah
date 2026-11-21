package acp

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"
)

// An answer may be one line of any length: a long chat's whole history comes
// back as one answer. A line above the reader's old limit of 64 MiB stopped
// the reader, and every later call on the connection then waited for ever.
func TestAnAnswerLongerThanTheOldLineLimitIsDelivered(t *testing.T) {
	fromAgent, agentOut := io.Pipe()
	agentIn, toAgent := io.Pipe()
	defer agentOut.Close()
	c := Connect(fromAgent, toAgent, closePipeHandler{}, Options{TraceHeadersOnly: true})
	defer c.Close()
	go io.Copy(io.Discard, agentIn)

	const size = 70 << 20
	second := make(chan struct{})
	go func() {
		io.WriteString(agentOut, `{"jsonrpc":"2.0","id":1,"result":{"text":"`)
		chunk := strings.Repeat("a", 1<<20)
		for written := 0; written < size; written += len(chunk) {
			io.WriteString(agentOut, chunk)
		}
		io.WriteString(agentOut, "\"}}\n")
		// A second, short answer, sent once its call is waiting, shows that
		// the reader goes on after the long one.
		<-second
		time.Sleep(50 * time.Millisecond)
		io.WriteString(agentOut, `{"jsonrpc":"2.0","id":2,"result":{"text":"after"}}`+"\n")
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	for i, want := range []int{size, len("after")} {
		if i == 1 {
			close(second)
		}
		var got struct {
			Text string `json:"text"`
		}
		if err := c.Call(ctx, "load", nil, &got); err != nil {
			t.Fatalf("call %d: %v", i+1, err)
		}
		if len(got.Text) != want {
			t.Fatalf("call %d: the answer has %d bytes of text, want %d", i+1, len(got.Text), want)
		}
	}
}

// A reader that ends while the agent's side stays open can deliver no answer,
// so a call that is waiting fails and the connection says it is done.
func TestACallFailsWhenTheReaderEnds(t *testing.T) {
	fromAgent, agentOut := io.Pipe()
	agentIn, toAgent := io.Pipe()
	c := Connect(fromAgent, toAgent, closePipeHandler{}, Options{TraceHeadersOnly: true})
	defer c.Close()
	go io.Copy(io.Discard, agentIn)

	failed := make(chan error, 1)
	go func() {
		failed <- c.Call(context.Background(), "load", nil, nil)
	}()
	time.Sleep(50 * time.Millisecond)
	agentOut.CloseWithError(io.ErrUnexpectedEOF)
	select {
	case err := <-failed:
		if err == nil {
			t.Fatal("the call returned no error after the reader ended")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the call still waited after the reader ended")
	}
	select {
	case <-c.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("the connection was not done after the reader ended")
	}
}
