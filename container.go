package dockertest

import (
	"context"
	"errors"
	"fmt"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/strslice"
	"github.com/docker/docker/client"
	"github.com/mohae/deepcopy"
	"strings"
	"time"
)

// Container is a access wrapper for a docker container
type Container struct {
	Name          string
	containerBody container.ContainerCreateCreatedBody
	StartOptions  types.ContainerStartOptions
	ctx           context.Context
	dockerClient  *client.Client
}

// Start starts the container.
func (c *Container) Start() error {
	return c.dockerClient.ContainerStart(c.ctx, c.containerBody.ID, c.StartOptions)
}

func (c *Container) ExitCode() (int, error) {
	insp, err := c.dockerClient.ContainerInspect(c.ctx, c.containerBody.ID)
	if err != nil {
		return -1, err
	}

	if insp.State.Running {
		return -1, errors.New("container is running, it has no exit code yet")
	}

	return insp.State.ExitCode, nil
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
	sessionId        string
}

func (b *ContainerBuilder) NewContainerBuilder() *ContainerBuilder {
	newBuilder := deepcopy.Copy(b).(*ContainerBuilder)
	newBuilder.ctx = b.ctx
	newBuilder.dockerClient = b.dockerClient
	newBuilder.sessionId = b.sessionId
	newBuilder.originalName = b.originalName

	return newBuilder
}

// Build creates a container from the current builders state.
func (b *ContainerBuilder) Build() (*Container, error) {

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

// Connect connects the container to the given network,
func (b *ContainerBuilder) Connect(n *Net) *ContainerBuilder {
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

// Cmd sets the command that is executed when the container starts
func (b *ContainerBuilder) Cmd(cmd string) *ContainerBuilder {
	b.ContainerConfig.Cmd = strslice.StrSlice(strings.Split(cmd, " "))
	return b
}
func (b *ContainerBuilder) Name(s string) *ContainerBuilder {
	b.originalName = s
	b.ContainerName = fmt.Sprintf("%s-%s", s, b.sessionId)
	return b
}

func (b *ContainerBuilder) AutoRemove(v bool) *ContainerBuilder {
	b.HostConfig.AutoRemove = v
	return b
}

func (b *ContainerBuilder) Image(image string) *ContainerBuilder {
	b.ContainerConfig.Image = image
	return b
}

func (b *ContainerBuilder) HealthShellCmd(cmd string) *ContainerBuilder {
	b.ContainerConfig.Healthcheck = &container.HealthConfig{
		Test:     []string{"CMD-SHELL", cmd},
		Interval: 200 * time.Millisecond,
		Retries:  20,
	}
	return b
}

//Env defines an environment variable that will be set in the container.
func (b *ContainerBuilder) Env(name string, value string) *ContainerBuilder {
	b.ContainerConfig.Env = append(b.ContainerConfig.Env, fmt.Sprintf("%s=%s", name, value))
	return b
}

// WorkingDir defines the working directory for the container.
func (b *ContainerBuilder) WorkingDir(wd string) *ContainerBuilder {
	b.ContainerConfig.WorkingDir = wd
	return b
}

// Dns adds a dns server to the container.
func (b *ContainerBuilder) Dns(dnsServerIP string) *ContainerBuilder {
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

// IPAddress defines the IP address used by the container.
func (b *ContainerBuilder) IPAddress(ipAddress string, n *Net) *ContainerBuilder {
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
