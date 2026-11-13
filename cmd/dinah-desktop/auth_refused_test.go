package main

import "testing"

// A provider that signs in outside the window is known to be signed in when
// the request it refused goes through, and by no other request.
func TestTheRefusedRequestGoingThroughEndsTheRefusal(t *testing.T) {
	var r refusedCalls
	if r.any() || r.answered("claude", "session/prompt") {
		t.Fatal("a refusal was found before any request was refused")
	}
	r.refuse("claude", "session/prompt")
	if !r.any() {
		t.Fatal("the refusal was not kept")
	}
	if r.answered("claude", "session/list") {
		t.Fatal("another request going through ended the refusal")
	}
	if r.answered("codex", "session/prompt") {
		t.Fatal("another provider's request ended the refusal")
	}
	if !r.answered("claude", "session/prompt") {
		t.Fatal("the refused request going through did not end the refusal")
	}
	if r.any() || r.answered("claude", "session/prompt") {
		t.Fatal("the refusal was ended twice")
	}
}

// Claude's module says how the person signs in, since the window cannot.
func TestClaudeSaysHowToSignIn(t *testing.T) {
	if help := providerModules["claude"].SignInHelp; help == "" || providerModules["claude"].AuthRequired != nil {
		t.Fatalf("Claude's sign-in help is %q", help)
	}
}
