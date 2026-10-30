package acp

import (
	"errors"
	"fmt"
	"testing"
)

func TestIsAuthRequired(t *testing.T) {
	cases := []struct {
		err  error
		want bool
	}{
		{&Error{Code: CodeAuthRequired, Message: "Authentication required"}, true},
		{fmt.Errorf("session/new: %w", &Error{Code: -32000}), true},
		{&Error{Code: -32002, Message: "Resource not found"}, false},
		{errors.New("the agent stopped"), false},
		{nil, false},
	}
	for _, c := range cases {
		if got := IsAuthRequired(c.err); got != c.want {
			t.Errorf("IsAuthRequired(%v) = %v, want %v", c.err, got, c.want)
		}
	}
}
