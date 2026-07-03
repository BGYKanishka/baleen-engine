package docker

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/docker/docker/api/types/image"
)

func TestLoadImage(t *testing.T) {
	loaded := false
	mockCli := &MockClient{
		LoadFn: func(ctx context.Context, input io.Reader, quiet bool) (image.LoadResponse, error) {
			loaded = true
			return image.LoadResponse{Body: io.NopCloser(strings.NewReader("dummy body"))}, nil
		},
	}
	manager := &Manager{Cli: mockCli}

	tempDir := t.TempDir()
	dummyFile := filepath.Join(tempDir, "dummy.tar")
	os.WriteFile(dummyFile, []byte("tarball"), 0644)

	err := manager.LoadImage(dummyFile)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if !loaded {
		t.Errorf("Expected ImageLoad to be called")
	}
}
