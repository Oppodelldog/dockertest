package dockertest

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/mohae/deepcopy"
)

var ErrContainerStillRunning = errors.New("container is running, it has no exit code yet")

// Container is a access wrapper for a docker container.
type Container struct {
	Name          string
	startOptions  types.ContainerStartOptions
	containerBody container.ContainerCreateCreatedBody
	ClientEnabled
}

// Start starts the container.
func (c *Container) Start() error {
	return c.dockerClient.ContainerStart(c.ctx, c.containerBody.ID, c.startOptions)
}

// ExitCode returns the exit code of the container.
// The container must be exited and exist, otherwise an error is returned.
func (c *Container) ExitCode() (int, error) {
	inspectResult, inspectError := c.dockerClient.ContainerInspect(c.ctx, c.containerBody.ID)
	if inspectError != nil {
		return -1, inspectError
	}

	if inspectResult.State.Running {
		return -1, ErrContainerStillRunning
	}

	return inspectResult.State.ExitCode, nil
}

// ContainerBuilder helps to create customized containers.
// Note that calling functions have not affect to running or already created container.
// only when calling the "Build" method all configuration is applied to a new container.
type ContainerBuilder struct {
	ContainerConfig  *container.Config
	HostConfig       *container.HostConfig
	NetworkingConfig *network.NetworkingConfig
	ContainerName    string
	originalName     string
	sessionID        string
	ClientEnabled
}

func (b *ContainerBuilder) NewContainerBuilder() *ContainerBuilder {
	newBuilder := deepcopy.Copy(b).(*ContainerBuilder)
	newBuilder.ctx = b.ctx
	newBuilder.dockerClient = b.dockerClient
	newBuilder.sessionID = b.sessionID
	newBuilder.originalName = b.originalName

	return newBuilder
}

// Build creates a container from the current builders state.
func (b *ContainerBuilder) Build() (*Container, error) {
	containerBody, err := b.dockerClient.ContainerCreate(
		b.ctx,
		b.ContainerConfig,
		b.HostConfig,
		b.NetworkingConfig,
		b.ContainerName,
	)
	if err != nil {
		return nil, err
	}

	return &Container{
		Name:          b.ContainerName,
		containerBody: containerBody,
		ClientEnabled: b.ClientEnabled,
	}, nil
}

// Connect connects the container to the given network.
func (b *ContainerBuilder) Connect(n *Network) *ContainerBuilder {
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

// Cmd sets the command that is executed when the container starts.
func (b *ContainerBuilder) Cmd(cmd string) *ContainerBuilder {
	b.ContainerConfig.Cmd = strings.Split(cmd, " ")
	return b
}

//Name defines the container name.
func (b *ContainerBuilder) Name(s string) *ContainerBuilder {
	b.originalName = s
	b.ContainerName = s

	if b.sessionID != "" {
		b.ContainerName = fmt.Sprintf("%s-%s", s, b.sessionID)
	}

	return b
}

//AutoRemove tells the docker daemon to remove the container after it exits.
func (b *ContainerBuilder) AutoRemove(v bool) *ContainerBuilder {
	b.HostConfig.AutoRemove = v
	return b
}

//Image sets the docker image to start a container from.
func (b *ContainerBuilder) Image(image string) *ContainerBuilder {
	b.ContainerConfig.Image = image
	return b
}

//HealthDisable disabled the health check.
func (b *ContainerBuilder) HealthDisable() *ContainerBuilder {
	b.ensureHealth()
	b.ContainerConfig.Healthcheck.Test = []string{"NONE"}

	return b
}

//HealthCmd sets a command that is executed directly.
func (b *ContainerBuilder) HealthCmd(cmd string) *ContainerBuilder {
	b.ensureHealth()
	b.ContainerConfig.Healthcheck.Test = []string{"CMD", cmd}

	return b
}

//HealthShellCmd sets a command that is executed in the containers default shell
//to determine if the container is healthy.
func (b *ContainerBuilder) HealthShellCmd(cmd string) *ContainerBuilder {
	b.ensureHealth()
	b.ContainerConfig.Healthcheck.Test = []string{"CMD-SHELL", cmd}

	return b
}

//HealthTimeout sets the timeout to wait before considering the check to have hung.
func (b *ContainerBuilder) HealthTimeout(t time.Duration) *ContainerBuilder {
	b.ensureHealth()
	b.ContainerConfig.Healthcheck.Timeout = t

	return b
}

//HealthInterval sets the time to wait between checks.
func (b *ContainerBuilder) HealthInterval(d time.Duration) *ContainerBuilder {
	b.ensureHealth()
	b.ContainerConfig.Healthcheck.Interval = d

	return b
}

//HealthRetries sets the number of consecutive failures needed to consider a container as unhealthy.
func (b *ContainerBuilder) HealthRetries(r int) *ContainerBuilder {
	b.ensureHealth()
	b.ContainerConfig.Healthcheck.Retries = r

	return b
}

func (b *ContainerBuilder) ensureHealth() {
	if b.ContainerConfig.Healthcheck == nil {
		b.ContainerConfig.Healthcheck = &container.HealthConfig{}
	}
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

// DNS adds a dns server to the container.
func (b *ContainerBuilder) DNS(dnsServerIP string) *ContainerBuilder {
	b.HostConfig.DNS = append(b.HostConfig.DNS, dnsServerIP)

	return b
}

// UseOriginalName removes the unique session-identifier from the container name.
func (b *ContainerBuilder) UseOriginalName() *ContainerBuilder {
	b.ContainerName = b.originalName

	return b
}

// Link links a foreign container.
func (b *ContainerBuilder) Link(container *Container, alias string, n *Network) *ContainerBuilder {
	b.ensureNetworkConfig(n)
	b.NetworkingConfig.EndpointsConfig[n.NetworkName].Links = append(
		b.NetworkingConfig.EndpointsConfig[n.NetworkName].Links,
		fmt.Sprintf("%s:%s", container.Name, alias),
	)

	return b
}

func (b *ContainerBuilder) ensureNetworkConfig(n *Network) {
	if b.NetworkingConfig.EndpointsConfig == nil {
		b.NetworkingConfig.EndpointsConfig = map[string]*network.EndpointSettings{}
	}

	if b.NetworkingConfig.EndpointsConfig[n.NetworkName] == nil {
		b.NetworkingConfig.EndpointsConfig[n.NetworkName] = &network.EndpointSettings{}
	}
}

// IPAddress defines the IP address used by the container.
func (b *ContainerBuilder) IPAddress(ipAddress string, n *Network) *ContainerBuilder {
	if b.NetworkingConfig.EndpointsConfig == nil {
		b.NetworkingConfig.EndpointsConfig = map[string]*network.EndpointSettings{}
	}

	if b.NetworkingConfig.EndpointsConfig[n.NetworkName] == nil {
		endpointSetting := &network.EndpointSettings{IPAMConfig: &network.EndpointIPAMConfig{IPv4Address: ipAddress}}
		b.NetworkingConfig.EndpointsConfig[n.NetworkName] = endpointSetting
	} else {
		b.NetworkingConfig.EndpointsConfig[n.NetworkName].IPAMConfig = &network.EndpointIPAMConfig{IPv4Address: ipAddress}
	}

	return b
}
