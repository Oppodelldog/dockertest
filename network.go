package dockertest

import (
	"github.com/docker/docker/api/types"
)

// Network represents a docker network.
type Network struct {
	NetworkID   string
	NetworkName string
}

// NetworkBuilder helps with the creation of a docker network.
type NetworkBuilder struct {
	Name    string
	Options types.NetworkCreate
	clientEnabled
}

// Create creates a new docker network.
func (n NetworkBuilder) Create() (*Network, error) {
	resp, err := n.dockerClient.NetworkCreate(n.ctx, n.Name, n.Options)
	if err != nil {
		return nil, err
	}

	return &Network{resp.ID, n.Name}, nil
}
