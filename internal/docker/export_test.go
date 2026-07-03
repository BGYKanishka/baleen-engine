package docker

import (
	"context"
	"io"
	"strings"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
)

type MockClient struct {
	client.APIClient
	InspectFn func(ctx context.Context, image string) (types.ImageInspect, []byte, error)
	SaveFn    func(ctx context.Context, imageIDs []string, opts ...client.ImageSaveOption) (io.ReadCloser, error)
	RemoveFn  func(ctx context.Context, image string, options image.RemoveOptions) ([]image.DeleteResponse, error)
	LoadFn    func(ctx context.Context, input io.Reader, quiet bool) (image.LoadResponse, error)
	TagFn     func(ctx context.Context, source, target string) error
}

// Implement the methods of the Docker client interface to call the corresponding function fields
func (m *MockClient) ImageInspectWithRaw(ctx context.Context, image string) (types.ImageInspect, []byte, error) {
	if m.InspectFn != nil {
		return m.InspectFn(ctx, image)
	}
	return types.ImageInspect{}, nil, nil
}

func (m *MockClient) ImageSave(ctx context.Context, imageIDs []string, opts ...client.ImageSaveOption) (io.ReadCloser, error) {
	if m.SaveFn != nil {
		return m.SaveFn(ctx, imageIDs, opts...)
	}
	return io.NopCloser(strings.NewReader("dummy tarball content")), nil
}

func (m *MockClient) ImageRemove(ctx context.Context, image string, options image.RemoveOptions) ([]image.DeleteResponse, error) {
	if m.RemoveFn != nil {
		return m.RemoveFn(ctx, image, options)
	}
	return nil, nil
}

func (m *MockClient) ImageLoad(ctx context.Context, input io.Reader, quiet ...client.ImageLoadOption) (image.LoadResponse, error) {
	if m.LoadFn != nil {
		return m.LoadFn(ctx, input, false)
	}
	return image.LoadResponse{}, nil
}

func (m *MockClient) ImageTag(ctx context.Context, source, target string) error {
	if m.TagFn != nil {
		return m.TagFn(ctx, source, target)
	}
	return nil
}
func TestExportImage(t *testing.T) {
	mockCli := &MockClient{
		InspectFn: func(ctx context.Context, image string) (types.ImageInspect, []byte, error) {
			return types.ImageInspect{
				Architecture: "amd64",
				Os:           "linux",
			}, nil, nil
		},
	}
	manager := &Manager{Cli: mockCli}

	cfg := PreflightConfig{
		ImageName:      "test-image",
		ExpectedTarget: "linux/amd64",
		ExportDir:      t.TempDir(),
	}

	tarball, arch, err := manager.ExportImage(cfg)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if arch != "linux/amd64" {
		t.Errorf("Expected arch linux/amd64, got %s", arch)
	}
	if tarball == "" {
		t.Errorf("Expected tarball path to be non-empty")
	}
}

func TestGetImageLayers(t *testing.T) {
	mockCli := &MockClient{
		InspectFn: func(ctx context.Context, image string) (types.ImageInspect, []byte, error) {
			return types.ImageInspect{
				RootFS: types.RootFS{
					Layers: []string{"layer1", "layer2"},
				},
			}, nil, nil
		},
	}
	manager := &Manager{Cli: mockCli}

	layers, err := manager.GetImageLayers("test-image")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if len(layers) != 2 || layers[0] != "layer1" {
		t.Errorf("Expected layers to be ['layer1', 'layer2'], got %v", layers)
	}
}
