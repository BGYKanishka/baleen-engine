package transfer

import (
	"bytes"
	"sync"
	"testing"
	"time"
)

type mockWriter struct {
	buf bytes.Buffer
}

func (m *mockWriter) Write(p []byte) (n int, err error) {
	return m.buf.Write(p)
}

func TestProgressWriter_Basic(t *testing.T) {
	mw := &mockWriter{}
	pw := newProgressWriter(mw, 100, "test-image", "peer1", "push")
	defer pw.Cleanup()

	if pw.Direction() != "push" {
		t.Errorf("expected direction 'push', got '%s'", pw.Direction())
	}

	n, err := pw.Write([]byte("hello"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 5 {
		t.Errorf("expected 5 bytes written, got %d", n)
	}
	if mw.buf.String() != "hello" {
		t.Errorf("expected 'hello', got '%s'", mw.buf.String())
	}
	if pw.currentProgress() != 5.0 {
		t.Errorf("expected 5.0%% progress, got %f", pw.currentProgress())
	}
}

func TestProgressWriter_PauseResume(t *testing.T) {
	mw := &mockWriter{}
	pw := newProgressWriter(mw, 100, "test-image", "peer1", "push")
	defer pw.Cleanup()

	// Capture signals
	var signals []string
	var mu sync.Mutex
	pw.notifyPeer = func(action, initiator string) {
		mu.Lock()
		defer mu.Unlock()
		signals = append(signals, action)
	}

	pw.Pause("sender")

	// Write while paused should block, so we run it in a goroutine
	writeDone := make(chan struct{})
	go func() {
		_, _ = pw.Write([]byte("test"))
		close(writeDone)
	}()

	// Ensure write is blocked
	select {
	case <-writeDone:
		t.Fatal("Write did not block while paused")
	case <-time.After(50 * time.Millisecond):
	}

	// Resume should unblock Write
	err := pw.Resume("sender")
	if err != nil {
		t.Fatalf("unexpected error on resume: %v", err)
	}

	select {
	case <-writeDone:
		// Success
	case <-time.After(50 * time.Millisecond):
		t.Fatal("Write still blocked after resume")
	}
	// signalPeer fires the notifyPeer callback in a goroutine;
	// give it a moment to land before checking collected signals.
	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	if len(signals) != 2 || signals[0] != "pause" || signals[1] != "resume" {
		t.Errorf("expected pause and resume signals, got %v", signals)
	}
	mu.Unlock()
}

func TestProgressWriter_InvalidResume(t *testing.T) {
	mw := &mockWriter{}
	pw := newProgressWriter(mw, 100, "test-image", "peer1", "push")
	defer pw.Cleanup()

	pw.Pause("sender")
	err := pw.Resume("receiver")
	if err == nil {
		t.Fatal("expected error when resuming with wrong initiator")
	}
}

func TestProgressWriter_Cancel(t *testing.T) {
	mw := &mockWriter{}
	pw := newProgressWriter(mw, 100, "test-image", "peer1", "push")
	defer pw.Cleanup()

	pw.Cancel()

	_, err := pw.Write([]byte("test"))
	if err == nil || err.Error() != "transfer cancelled" {
		t.Fatalf("expected 'transfer cancelled' error, got: %v", err)
	}
}

func TestProgressWriter_CancelWhilePaused(t *testing.T) {
	mw := &mockWriter{}
	pw := newProgressWriter(mw, 100, "test-image", "peer1", "push")
	defer pw.Cleanup()

	pw.Pause("sender")

	writeDone := make(chan error, 1)
	go func() {
		_, err := pw.Write([]byte("test"))
		writeDone <- err
	}()

	// Wait for goroutine to block
	time.Sleep(50 * time.Millisecond)

	pw.Cancel()

	select {
	case err := <-writeDone:
		if err == nil || err.Error() != "transfer cancelled" {
			t.Fatalf("expected 'transfer cancelled', got %v", err)
		}
	case <-time.After(50 * time.Millisecond):
		t.Fatal("Write did not unblock after cancel")
	}
}

func TestProgressWriter_Notify(t *testing.T) {
	mw := &mockWriter{}
	pw := newProgressWriter(mw, 100, "test-image", "peer1", "push")
	defer pw.Cleanup()

	pw.NotifyPausedBy("receiver")
	pw.mu.Lock()
	got := pw.pausedBy
	pw.mu.Unlock()
	if got != "receiver" {
		t.Errorf("expected pausedBy 'receiver', got %s", got)
	}

	pw.NotifyResumed()
	pw.mu.Lock()
	got = pw.pausedBy
	pw.mu.Unlock()
	if got != "" {
		t.Errorf("expected pausedBy '', got %s", got)
	}
}

type mockControlConn struct {
	written []byte
}

func (m *mockControlConn) Write(p []byte) (n int, err error) {
	m.written = append(m.written, p...)
	return len(p), nil
}

func TestProgressWriter_ControlConnSignal(t *testing.T) {
	mw := &mockWriter{}
	pw := newProgressWriter(mw, 100, "test-image", "peer1", "pull")
	defer pw.Cleanup()

	conn := &mockControlConn{}
	pw.ControlConn = conn

	// Pull direction, local initiator is "receiver"
	pw.Pause("receiver")
	if len(conn.written) != 1 || conn.written[0] != 'P' {
		t.Errorf("expected 'P' signal on ControlConn, got %v", conn.written)
	}

	pw.Resume("receiver")
	if len(conn.written) != 2 || conn.written[1] != 'R' {
		t.Errorf("expected 'R' signal on ControlConn, got %v", conn.written)
	}

	pw.Cancel()
	if len(conn.written) != 3 || conn.written[2] != 'C' {
		t.Errorf("expected 'C' signal on ControlConn, got %v", conn.written)
	}
}
