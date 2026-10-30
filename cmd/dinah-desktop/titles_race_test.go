package main

import (
	"encoding/json"
	"fmt"
	"sync"
	"testing"
)

// A flow names several chats at once while the page reads the state. The
// titles each call returns are encoded after the app's lock is released, so
// they must be copies: encoding the app's own map while another chat is named
// ended the program with "concurrent map read and map write" (15 October
// 2026, four chapters drafted side by side).
func TestChatsNamedSideBySideWhileTheStateIsRead(t *testing.T) {
	a, _ := providerTestApp(t)
	var wg sync.WaitGroup
	for w := 0; w < 4; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for i := 0; i < 40; i++ {
				titles, _ := a.RenameSession(fmt.Sprintf("chat-%d-%d", w, i), fmt.Sprintf("Flow: Draft chapter %d", i))
				if _, err := json.Marshal(titles); err != nil {
					t.Error(err)
				}
			}
		}(w)
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 80; i++ {
			if _, err := json.Marshal(a.State()); err != nil {
				t.Error(err)
			}
		}
	}()
	wg.Wait()
	titles, _ := a.RenameSession("last", "Last")
	if len(titles) != 161 {
		t.Fatalf("%d titles, want 161", len(titles))
	}
}
