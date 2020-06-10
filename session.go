package dockertest

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
)

var ErrContainerStartTimeout = errors.New("timeout - container is not healthy")

const cleanerTimeout = 10 * time.Second
const mainLabel = "dockertest"
const mainLabelValue = "dockertest"
const sessionLabel = "docker-dns-session"

// NewSession creates a new Test and returns a Session instance to work with.
func NewSession() (*Session, error) {
	sessionID := time.Now().Format("20060102150405")

	dockerClient, err := client.NewEnvClient()
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &Session{
		ID: sessionID,
		ClientEnabled: ClientEnabled{
			cancelCtx:    cancel,
			ctx:          ctx,
			dockerClient: dockerClient,
		},
	}, nil
}

//Session is the main object when starting a docker driven container test.
type Session struct {
	ID     string
	logDir string
	ClientEnabled
}

func panicOnError(err error) {
	if err != nil {
		panic(err)
	}
}

// SetLogDir sets the directory for log files creating during test execution.
// When calling it will directly ensure the path.
func (dt *Session) SetLogDir(logDir string) {
	err := os.MkdirAll(logDir, 0777)
	panicOnError(err)

	dt.logDir = logDir
}

// WaitForContainerToExit returns a channel that blocks until the container has exited.
// If the operation times out, it will try to kill the container.
func (dt *Session) WaitForContainerToExit(container *Container, timeout time.Duration) chan bool {
	exitedCh := make(chan bool)

	go func() {
		ctxTimeout, cancel := context.WithTimeout(dt.ctx, timeout)
		defer cancel()

		if !waitForContainer(ctxTimeout, containerHasFadeAway, dt.dockerClient, container.containerBody.ID) {
			err := dt.dockerClient.ContainerKill(context.Background(), container.containerBody.ID, "kill")
			if err != nil {
				fmt.Println("Error while killing container,", err)
			}
		}
		exitedCh <- true
	}()

	return exitedCh
}

// WaitForContainerToBeHealthy returns a channel that blocks until the given
// container reaches healthy state or timeout occurrs.
func (dt *Session) WaitForContainerToBeHealthy(container *Container, timeout time.Duration) chan error {
	healthErr := make(chan error)

	go func() {
		ctxTimeout, cancel := context.WithTimeout(dt.ctx, timeout)
		defer cancel()

		if !waitForContainer(ctxTimeout, containerIsHealthy, dt.dockerClient, container.containerBody.ID) {
			healthErr <- ErrContainerStartTimeout
		}
		healthErr <- nil
	}()

	return healthErr
}

// Cleanup removes all resources (like containers/networks) used for the test.
func (dt *Session) Cleanup() {
	ctx, cancel := context.WithTimeout(context.Background(), cleanerTimeout)
	defer cancel()

	cleaner := newCleaner(ctx, dt)
	cleaner.stopSessionContainers(dt.ID)
	cleaner.removeDockerTestContainers(dt.ID)
	cleaner.cleanupTestNetwork()
}

func (dt *Session) getLabels() map[string]string {
	return map[string]string{
		mainLabel:    mainLabelValue,
		sessionLabel: dt.ID,
	}
}

// Start starts one or multiple given containers.
// If some containers return error while starting the last error will be returned.
func (dt *Session) StartContainer(container ...*Container) error {
	var err error

	for i := range container {
		errStart := container[i].Start()
		if errStart != nil {
			err = errStart
		}
	}

	return err
}

// DumpInspect dumps an json file with the content of "docker inspect" into the log directory.
func (dt *Session) DumpInspect(container ...*Container) {
	for _, c := range container {
		dumpInspectContainter(dt.ctx, dt.dockerClient, c, dt.logDir)
	}
}

// DumpContainerLogs dumps the log of one or multiple containers to the log directory.
func (dt *Session) DumpContainerLogs(container ...*Container) {
	for _, c := range container {
		dumpContainerLog(dt.ctx, dt.dockerClient, c, dt.logDir)
	}
}

// CreateSimpleNetwork creates a bridged network with the given name, subnet mask and ip range.
func (dt *Session) CreateBasicNetwork(networkName string) *NetworkBuilder {
	ctx, cancel := context.WithTimeout(context.Background(), cleanerTimeout)
	defer cancel()

	cleaner := newCleaner(ctx, dt)
	cleaner.cleanupTestNetwork()

	return &NetworkBuilder{
		ClientEnabled: dt.ClientEnabled,
		Name:          networkName,
		Options: types.NetworkCreate{
			CheckDuplicate: true,
			Attachable:     true,
			Driver:         "bridge",
			IPAM: &network.IPAM{
				Driver: "default",
				Config: []network.IPAMConfig{},
			},
			Labels: dt.getLabels(),
		},
	}
}

// CreateSimpleNetwork creates a bridged network with the given name, subnet mask and ip range.
func (dt *Session) CreateSimpleNetwork(networkName, subNet, ipRange string) *NetworkBuilder {
	ctx, cancel := context.WithTimeout(context.Background(), cleanerTimeout)
	defer cancel()

	cleaner := newCleaner(ctx, dt)
	cleaner.cleanupTestNetwork()

	return &NetworkBuilder{
		ClientEnabled: dt.ClientEnabled,
		Name:          networkName,
		Options: types.NetworkCreate{
			CheckDuplicate: true,
			Attachable:     true,
			Driver:         "bridge",
			IPAM: &network.IPAM{
				Driver: "default",
				Config: []network.IPAMConfig{
					{
						Subnet:  subNet,
						IPRange: ipRange,
					},
				},
			},
			Labels: dt.getLabels(),
		},
	}
}

func (dt *Session) NewContainerBuilder() *ContainerBuilder {
	return &ContainerBuilder{
		ClientEnabled: dt.ClientEnabled,
		sessionID:     dt.ID,
		ContainerConfig: &container.Config{
			Labels: dt.getLabels(),
		},
		NetworkingConfig: &network.NetworkingConfig{},
		HostConfig:       &container.HostConfig{},
	}
}

func getBasicFilterArgs() filters.Args {
	filterArgs := filters.NewArgs()
	filterArgs.Add("label", fmt.Sprintf("%s=%s", mainLabel, mainLabelValue))

	return filterArgs
}
