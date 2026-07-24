//go:build windows

package backend

import (
	"syscall"
	"testing"
	"time"
)

func TestDamageOverlayStopCompletesBeforeRestart(t *testing.T) {
	overlay := newDamageOverlayWindow()
	closePosted := make(chan struct{})
	allowFirstExit := make(chan struct{})
	firstDone := make(chan struct{})
	firstRunner := func(ready chan<- error, done chan<- struct{}) {
		overlay.mu.Lock()
		overlay.hwnd = 1
		overlay.mu.Unlock()
		ready <- nil
		<-allowFirstExit
		overlay.mu.Lock()
		overlay.hwnd = 0
		overlay.mu.Unlock()
		close(done)
		close(firstDone)
	}
	if err := overlay.startWithRunner(firstRunner); err != nil {
		t.Fatal(err)
	}

	stopResult := make(chan error, 1)
	go func() {
		stopResult <- overlay.stopWithPoster(func(hwnd syscall.Handle) error {
			if hwnd != 1 {
				t.Errorf("posted close to hwnd=%d, want 1", hwnd)
			}
			close(closePosted)
			return nil
		}, time.Second)
	}()
	<-closePosted

	secondStarted := make(chan struct{})
	allowSecondExit := make(chan struct{})
	secondDone := make(chan struct{})
	startResult := make(chan error, 1)
	go func() {
		startResult <- overlay.startWithRunner(func(ready chan<- error, done chan<- struct{}) {
			overlay.mu.Lock()
			overlay.hwnd = 2
			overlay.mu.Unlock()
			close(secondStarted)
			ready <- nil
			<-allowSecondExit
			overlay.mu.Lock()
			overlay.hwnd = 0
			overlay.mu.Unlock()
			close(done)
			close(secondDone)
		})
	}()

	select {
	case <-secondStarted:
		t.Fatal("restart began before the previous overlay finished closing")
	case <-time.After(50 * time.Millisecond):
	}
	close(allowFirstExit)
	<-firstDone
	if err := <-stopResult; err != nil {
		t.Fatal(err)
	}
	select {
	case <-secondStarted:
	case <-time.After(time.Second):
		t.Fatal("restart did not begin after the previous overlay closed")
	}
	if err := <-startResult; err != nil {
		t.Fatal(err)
	}
	close(allowSecondExit)
	<-secondDone
}
