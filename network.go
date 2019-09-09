package dockertest

import (
	"context"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
)

type Network struct {
	NetworkID   string
	NetworkName string
}

type NetworkBuilder struct {
	Name         string
	Options      types.NetworkCreate
	dockerClient *client.Client
	ctx          context.Context
}

func (n *NetworkBuilder) Create() (*Network, error) {
	resp, err := n.dockerClient.NetworkCreate(n.ctx, n.Name, n.Options)
	if err != nil {
		return nil, err
	}

	return &Network{resp.ID, n.Name}, nil
}
