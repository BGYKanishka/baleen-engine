package docker

import (
	"github.com/docker/docker/client"
)

// Manager is a wrapper around the Docker client that provides methods to interact with the Docker daemon
type Manager struct {
	Cli client.APIClient
}

// NewManager creates a new Manager instance connected to the local Docker daemon
func NewManager() (*Manager, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, err
	}
	return &Manager{Cli: cli}, nil
}
