package service

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// Helpers

// overrides HOME and USERPROFILE to a temporary directory for testing.
func overrideHome(t *testing.T) string {
	t.Helper()
	tmp := t.TempDir()
	baleenDir := filepath.Join(tmp, ".baleen")
	if err := os.MkdirAll(baleenDir, 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", tmp)
	t.Setenv("USERPROFILE", tmp) // Windows
	return baleenDir
}

// State tests
func TestWriteAndReadState(t *testing.T) {
	overrideHome(t)

	want := ServiceState{
		Port:      12345,
		Token:     "test-token-abc",
		PID:       os.Getpid(),
		StartedAt: time.Now().Truncate(time.Second),
	}

	if err := WriteState(want); err != nil {
		t.Fatalf("WriteState: %v", err)
	}

	got, err := ReadState()
	if err != nil {
		t.Fatalf("ReadState: %v", err)
	}

	if got.Port != want.Port {
		t.Errorf("Port: got %d, want %d", got.Port, want.Port)
	}
	if got.Token != want.Token {
		t.Errorf("Token: got %q, want %q", got.Token, want.Token)
	}
	if got.PID != want.PID {
		t.Errorf("PID: got %d, want %d", got.PID, want.PID)
	}
}

func TestClearState(t *testing.T) {
	overrideHome(t)

	if err := WriteState(ServiceState{Port: 9999, Token: "tok", PID: os.Getpid()}); err != nil {
		t.Fatal(err)
	}

	ClearState()

	if _, err := ReadState(); !os.IsNotExist(err) {
		t.Errorf("expected file-not-found after ClearState; got err=%v", err)
	}
}

func TestReadState_Missing(t *testing.T) {
	overrideHome(t)
	// No WriteState called — file doesn't exist.
	_, err := ReadState()
	if err == nil {
		t.Fatal("expected error reading non-existent state, got nil")
	}
}

// IsAlive tests
func TestIsAlive_CurrentProcess(t *testing.T) {
	state := ServiceState{PID: os.Getpid()}
	if !IsAlive(state) {
		t.Error("IsAlive should return true for the current process PID")
	}
}

func TestIsAlive_DeadPID(t *testing.T) {
	// PID 0 is invalid on all platforms.
	state := ServiceState{PID: 0}
	if IsAlive(state) {
		t.Error("IsAlive should return false for PID 0")
	}
}

// WaitForReady tests
func TestWaitForReady_ImmediatelyAvailable(t *testing.T) {
	overrideHome(t)

	want := ServiceState{Port: 54321, Token: "ready-tok", PID: os.Getpid()}
	if err := WriteState(want); err != nil {
		t.Fatal(err)
	}

	got, err := WaitForReady(5 * time.Second)
	if err != nil {
		t.Fatalf("WaitForReady: %v", err)
	}
	if got.Port != want.Port {
		t.Errorf("Port: got %d, want %d", got.Port, want.Port)
	}
}

func TestWaitForReady_Timeout(t *testing.T) {
	overrideHome(t)
	// Never write the state file.
	_, err := WaitForReady(300 * time.Millisecond)
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
}

func TestWaitForReady_WrittenAfterDelay(t *testing.T) {
	overrideHome(t)

	want := ServiceState{Port: 11111, Token: "delayed", PID: os.Getpid()}
	go func() {
		time.Sleep(400 * time.Millisecond)
		WriteState(want)
	}()

	got, err := WaitForReady(5 * time.Second)
	if err != nil {
		t.Fatalf("WaitForReady: %v", err)
	}
	if got.Port != want.Port {
		t.Errorf("Port: got %d, want %d", got.Port, want.Port)
	}
}

// Lock tests
func TestTryAcquireLock_Fresh(t *testing.T) {
	overrideHome(t)

	ok, err := TryAcquireLock()
	if err != nil {
		t.Fatalf("TryAcquireLock: %v", err)
	}
	if !ok {
		t.Error("expected to acquire fresh lock, got false")
	}
	ReleaseLock()
}

func TestTryAcquireLock_StaleLock(t *testing.T) {
	overrideHome(t)

	// Write a stale PID (99999 is almost certainly not a running process, but
	// we write PID 0 which is always invalid to be portable).
	path, _ := lockPath()
	os.WriteFile(path, []byte("0"), 0600)

	ok, err := TryAcquireLock()
	if err != nil {
		t.Fatalf("TryAcquireLock with stale lock: %v", err)
	}
	if !ok {
		t.Error("expected to acquire stale lock, got false")
	}
	ReleaseLock()
}

func TestReleaseLock(t *testing.T) {
	overrideHome(t)

	if _, err := TryAcquireLock(); err != nil {
		t.Fatal(err)
	}
	ReleaseLock()

	path, _ := lockPath()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("lock file should not exist after ReleaseLock")
	}
}
