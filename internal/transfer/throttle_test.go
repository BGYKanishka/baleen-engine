package transfer

import (
	"bytes"
	"io"
	"testing"
	"time"
)

func TestThrottledReader_Unlimited(t *testing.T) {
	data := []byte("hello world, this is a test payload for throttling")
	reader := bytes.NewReader(data)

	tr := NewThrottledReader(reader, 0)

	buf := make([]byte, len(data))
	start := time.Now()
	n, err := io.ReadFull(tr, buf)
	duration := time.Since(start)

	if err != nil && err != io.EOF {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != len(data) {
		t.Fatalf("expected to read %d bytes, got %d", len(data), n)
	}

	// Should be almost instantaneous
	if duration > 100*time.Millisecond {
		t.Fatalf("unlimited reader took too long: %v", duration)
	}
}

func TestThrottledReader_Throttled(t *testing.T) {
	data := bytes.Repeat([]byte("a"), 150)
	reader := bytes.NewReader(data)

	tr := NewThrottledReader(reader, 50)

	buf := make([]byte, len(data))
	start := time.Now()
	n, err := io.ReadFull(tr, buf)
	duration := time.Since(start)

	if err != nil && err != io.EOF {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != len(data) {
		t.Fatalf("expected to read %d bytes, got %d", len(data), n)
	}
	if duration < 1500*time.Millisecond {
		t.Fatalf("throttled reader was too fast, took: %v", duration)
	}
}
