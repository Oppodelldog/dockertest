package dockertest

import (
	"context"
	"fmt"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
)

type Container struct {
	Name          string
	containerBody container.ContainerCreateCreatedBody
	StartOptions  types.ContainerStartOptions
	ctx           context.Context
	dockerClient  *client.Client
}

func (c *Container) StartContainer() error {
	return c.dockerClient.ContainerStart(c.ctx, c.containerBody.ID, c.StartOptions)
}

type ContainerBuilder struct {
	ContainerConfig  *container.Config
	HostConfig       *container.HostConfig
	NetworkingConfig *network.NetworkingConfig
	dockerClient     *client.Client
	ContainerName    string
	originalName     string
	ctx              context.Context
}

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

func (b *ContainerBuilder) ConnectToNetwork(n *Net) *ContainerBuilder {

	b.HostConfig.NetworkMode = container.NetworkMode(n.NetworkName)
	if b.NetworkingConfig.EndpointsConfig == nil {
		b.NetworkingConfig.EndpointsConfig = map[string]*network.EndpointSettings{}
	}
	endpointSetting := &network.EndpointSettings{
		NetworkID: n.NetworkID,
	}
	if b.NetworkingConfig.EndpointsConfig[n.NetworkName] == nil {
		b.NetworkingConfig.EndpointsConfig[n.NetworkName] = endpointSetting
	} else {
		b.NetworkingConfig.EndpointsConfig[n.NetworkName].NetworkID = n.NetworkID
	}
	return b
}

func (b *ContainerBuilder) Mount(localPath string, containerPath string) *ContainerBuilder {
	b.HostConfig.Binds = append(b.HostConfig.Binds, fmt.Sprintf("%s:%s", localPath, containerPath))
	return b
}

func (b *ContainerBuilder) SetEnv(name string, value string) *ContainerBuilder {
	b.ContainerConfig.Env = append(b.ContainerConfig.Env, fmt.Sprintf("%s=%s", name, value))
	return b
}

func (b *ContainerBuilder) SetWorkingDir(wd string) *ContainerBuilder {
	b.ContainerConfig.WorkingDir = wd
	return b
}

func (b *ContainerBuilder) AddDns(dnsServerIP string) *ContainerBuilder {
	b.HostConfig.DNS = append(b.HostConfig.DNS, dnsServerIP)
	return b
}

func (b *ContainerBuilder) UseOriginalName() *ContainerBuilder {
	b.ContainerName = b.originalName
	return b
}

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
