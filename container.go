package dockertest

import (
	"context"
	"fmt"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
)

// Container is a access wrapper for a docker container
type Container struct {
	Name          string
	containerBody container.ContainerCreateCreatedBody
	StartOptions  types.ContainerStartOptions
	ctx           context.Context
	dockerClient  *client.Client
}

// StartContainer starts the container.
func (c *Container) StartContainer() error {
	return c.dockerClient.ContainerStart(c.ctx, c.containerBody.ID, c.StartOptions)
}

// ContainerBuilder helps to create customized containers
type ContainerBuilder struct {
	ContainerConfig  *container.Config
	HostConfig       *container.HostConfig
	NetworkingConfig *network.NetworkingConfig
	dockerClient     *client.Client
	ContainerName    string
	originalName     string
	ctx              context.Context
}

// CreateContainer creates a container from the current builders state.
func (b *ContainerBuilder) CreateContainer() (*Container, error) {
	containerBody, err := b.dockerClient.ContainerCreate(b.ctx, b.ContainerConfig, b.HostConfig, b.NetworkingConfig, b.ContainerName)
	if err != nil {
		return nil, err
	}
	return &Container{
		Name:          b.ContainerName,
		containerBody: containerBody,
		ctx:           b.ctx,
		dockerClient:  b.dockerClient,
	}, nil
}

// ConnectToNetwork connects the container to the given network,
func (b *ContainerBuilder) ConnectToNetwork(n *Net) *ContainerBuilder {
	b.HostConfig.NetworkMode = container.NetworkMode(n.NetworkName)
	b.ensureNetworkConfig(n)
	b.NetworkingConfig.EndpointsConfig[n.NetworkName].NetworkID = n.NetworkID

	return b
}

// Mount creates a volume binding to mount a local directory into the container.
func (b *ContainerBuilder) Mount(localPath string, containerPath string) *ContainerBuilder {
	b.HostConfig.Binds = append(b.HostConfig.Binds, fmt.Sprintf("%s:%s", localPath, containerPath))
	return b
}

//SetEnv defines an environment variable that will be set in the container.
func (b *ContainerBuilder) SetEnv(name string, value string) *ContainerBuilder {
	b.ContainerConfig.Env = append(b.ContainerConfig.Env, fmt.Sprintf("%s=%s", name, value))
	return b
}

// SetWorkingDir defines the working directory for the container.
func (b *ContainerBuilder) SetWorkingDir(wd string) *ContainerBuilder {
	b.ContainerConfig.WorkingDir = wd
	return b
}

// AddDns adds a dns server to the container.
func (b *ContainerBuilder) AddDns(dnsServerIP string) *ContainerBuilder {
	b.HostConfig.DNS = append(b.HostConfig.DNS, dnsServerIP)
	return b
}

// UseOriginalName removes the unique session-identifier from the container name.
func (b *ContainerBuilder) UseOriginalName() *ContainerBuilder {
	b.ContainerName = b.originalName
	return b
}

// Link links a foreign container.
func (b *ContainerBuilder) Link(container *Container, alias string, n *Net) *ContainerBuilder {
	b.ensureNetworkConfig(n)
	links := b.NetworkingConfig.EndpointsConfig[n.NetworkName].Links
	b.NetworkingConfig.EndpointsConfig[n.NetworkName].Links = append(links, fmt.Sprintf("%s:%s", container.Name, alias))
	return b
}

func (b *ContainerBuilder) ensureNetworkConfig(n *Net) {
	if b.NetworkingConfig.EndpointsConfig == nil {
		b.NetworkingConfig.EndpointsConfig = map[string]*network.EndpointSettings{}
	}
	if b.NetworkingConfig.EndpointsConfig[n.NetworkName] == nil {
		b.NetworkingConfig.EndpointsConfig[n.NetworkName] = &network.EndpointSettings{}
	}
}

// SetIPAddress defines the IP address used by the container.
func (b *ContainerBuilder) SetIPAddress(ipAddress string, n *Net) *ContainerBuilder {
	if b.NetworkingConfig.EndpointsConfig == nil {
		b.NetworkingConfig.EndpointsConfig = map[string]*network.EndpointSettings{}
	}
	if b.NetworkingConfig.EndpointsConfig[n.NetworkName] == nil {
		b.NetworkingConfig.EndpointsConfig[n.NetworkName] = &network.EndpointSettings{IPAMConfig: &network.EndpointIPAMConfig{IPv4Address: ipAddress}}
	} else {
		b.NetworkingConfig.EndpointsConfig[n.NetworkName].IPAMConfig = &network.EndpointIPAMConfig{IPv4Address: ipAddress}
	}

	return b
}
