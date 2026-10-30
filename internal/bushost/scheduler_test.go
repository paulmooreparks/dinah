package bushost

import (
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestSchedulerQueueLimitsRetryAndCopies(t *testing.T) {
	var s Scheduler
	now := time.Now()
	s.SetWindows(nil)
	s.Expire(now)
	paths := map[string]string{}
	for i := 0; i < 5; i++ {
		id := fmt.Sprint(i)
		paths[id] = "folder"
		if _, ok := s.Consider(id, "folder", 1, false, false, now); ok {
			t.Fatal("background connected without admission")
		}
	}
	for i := 0; i < 3; i++ {
		d, ok := s.Next(paths, 0, now)
		if !ok || d.Chat != fmt.Sprint(i) || !d.Background {
			t.Fatal(d, ok)
		}
	}
	if _, ok := s.Next(paths, 0, now); ok {
		t.Fatal("exceeded slots")
	}
	failed := errors.New("failure")
	s.Complete("0", failed, now)
	if s.FailedSince("0", now) != failed || s.FailedSince("0", now.Add(time.Second)) != nil {
		t.Fatal("failure clock changed")
	}
	if d, ok := s.Next(paths, 0, now); !ok || d.Chat != "3" {
		t.Fatal("FIFO changed", d)
	}
	s.Expire(now.Add(ConnectPatience))
	s.SetWindows([]string{"0"})
	_, copy := s.Windows()
	delete(copy, "0")
	if !s.HasWindow("0") {
		t.Fatal("window map leaked")
	}
	if _, ok := s.Consider("0", "folder", 1, false, false, now); ok {
		t.Fatal("old message retried")
	}
	if d, ok := s.Consider("0", "folder", 2, false, false, now); !ok || d.Background {
		t.Fatal("new message did not reconnect window", d)
	}
	s.Complete("0", nil, now)
	if s.FailedSince("0", now) != nil {
		t.Fatal("failure was not cleared")
	}
}

func TestSchedulerAdmissionSharesSessionTransaction(t *testing.T) {
	var mu sync.Mutex
	var s Scheduler
	now := time.Now()
	s.SetWindows(nil)
	s.Expire(now)
	paths := map[string]string{}
	live := map[string]bool{}
	for i := 0; i < 30; i++ {
		id := fmt.Sprint(i)
		paths[id] = "folder"
		s.Consider(id, "folder", 1, false, false, now)
	}
	count := func() int {
		n := 0
		for _, v := range live {
			if v {
				n++
			}
		}
		return n
	}
	check := func() {
		if n := count() + s.BackgroundPending(); n > BackgroundSlots {
			t.Errorf("combined slot bound exceeded: %d", n)
		}
	}
	for i := 0; i < 3; i++ {
		s.Next(paths, 0, now)
	}
	var wg sync.WaitGroup
	for i := 0; i < 30; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			mu.Lock()
			c := s.Complete(fmt.Sprint(i), nil, now)
			if c.Asked && c.Background && !c.Window {
				live[fmt.Sprint(i)] = true
			}
			check()
			mu.Unlock()
			mu.Lock()
			s.Next(paths, count(), now)
			check()
			mu.Unlock()
			mu.Lock()
			s.SetWindows([]string{fmt.Sprint(i)})
			live[fmt.Sprint(i)] = false
			check()
			mu.Unlock()
		}(i)
	}
	wg.Wait()
}
