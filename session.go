package dockertest

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	dockerNetwork "github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
)

// ErrContainerStartTimeout is returned from NotifyContainerHealthy when a container
// was detected to be not healthy due to timeout.
var ErrContainerStartTimeout = errors.New("timeout - container is not healthy")

const cleanerTimeout = 10 * time.Second
const mainLabel = "dockertest"
const sessionLabel = mainLabel + "-session"
const defaultMainLabelValue = "dockertest"

// NewSession creates a new Test and returns a Session instance to work with.
func NewSession() (*Session, error) {
	sessionID := time.Now().Format("20060102150405")

	dockerClient, err := client.NewEnvClient()
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &Session{
		ID:        sessionID,
		mainLabel: defaultMainLabelValue,
		clientEnabled: clientEnabled{
			cancelCtx:    cancel,
			ctx:          ctx,
			dockerClient: dockerClient,
		},
	}, nil
}

//Session is the main object when starting a docker driven container test.
type Session struct {
	ID        string
	logDir    string
	mainLabel string
	clientEnabled
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

// SetLabel sets the label all components of this session are assigned to.
// it must be set before any further creation calls to take effect.
func (dt *Session) SetLabel(label string) {
	dt.mainLabel = label
}

// NotifyContainerExit returns a channel that blocks until the container has exited.
// If the operation times out, it will try to kill the container.
func (dt *Session) NotifyContainerExit(container *Container, timeout time.Duration) chan bool {
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

// NotifyContainerHealthy returns a channel that blocks until the given
// container reaches healthy state or timeout occurrs.
func (dt *Session) NotifyContainerHealthy(container *Container, timeout time.Duration) chan error {
	healthErr := make(chan error)

	go func() {
		ctxTimeout, cancel := context.WithTimeout(dt.ctx, timeout)
		defer cancel()

		if !waitForContainer(ctxTimeout, containerIsHealthy, dt.dockerClient, container.containerBody.ID) {
			healthErr <- fmt.Errorf("%w. timed out after %s", ErrContainerStartTimeout, timeout)
		}
		healthErr <- nil
	}()

	return healthErr
}

// Cleanup removes all resources (like containers/networks) used for this session.
func (dt *Session) Cleanup() {
	ctx, cancel := context.WithTimeout(context.Background(), cleanerTimeout)
	defer cancel()

	cleaner := newCleaner(ctx, dt)
	cleaner.stopSessionContainers(dt.ID)
	cleaner.removeDockerTestContainers(dt.ID)
	cleaner.cleanupTestNetwork()
}

// CleanupRemains removes all resources (like containers/networks) this kind of test - identified by the Session Label.
func (dt *Session) CleanupRemains() {
	c := newRemainsCleaner(dt.ctx, dt.dockerClient)
	c.stopContainers()
	c.removeDockerTestContainers()
	c.cleanupTestNetwork()
}

func (dt *Session) getLabels() map[string]string {
	return map[string]string{
		mainLabel:    dt.mainLabel,
		sessionLabel: dt.ID,
	}
}

// StartContainer starts one or multiple given containers.
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

// DumpContainerLogsToDir dumps the log of one or multiple containers to the log directory.
func (dt *Session) DumpContainerLogsToDir(container ...*Container) {
	for _, c := range container {
		dumpContainerLog(dt.ctx, dt.dockerClient, c, dt.logDir)
	}
}

// WriteContainerLogs writes the log of the given containers.
func (dt *Session) WriteContainerLogs(w io.Writer, container ...*Container) {
	for _, c := range container {
		log, err := getContainerLog(dt.ctx, dt.dockerClient, c)
		if err != nil {
			fmt.Printf("error writing container '%s' log: %v", c.Name, err)
			continue
		}

		writeLog(w, c, log)
	}
}

// CreateBasicNetwork creates a bridged Network with the given name, subnet mask and ip range.
func (dt *Session) CreateBasicNetwork(networkName string) NetworkBuilder {
	ctx, cancel := context.WithTimeout(context.Background(), cleanerTimeout)
	defer cancel()

	cleaner := newCleaner(ctx, dt)
	cleaner.cleanupTestNetwork()

	return NetworkBuilder{
		clientEnabled: dt.clientEnabled,
		Name:          networkName,
		Options: types.NetworkCreate{
			CheckDuplicate: true,
			Attachable:     true,
			Driver:         "bridge",
			IPAM: &dockerNetwork.IPAM{
				Driver: "default",
				Config: []dockerNetwork.IPAMConfig{},
			},
			Labels: dt.getLabels(),
		},
	}
}

// CreateSimpleNetwork creates a bridged Network with the given name, subnet mask and ip range.
func (dt *Session) CreateSimpleNetwork(networkName, subNet, ipRange string) NetworkBuilder {
	ctx, cancel := context.WithTimeout(context.Background(), cleanerTimeout)
	defer cancel()

	cleaner := newCleaner(ctx, dt)
	cleaner.cleanupTestNetwork()

	return NetworkBuilder{
		clientEnabled: dt.clientEnabled,
		Name:          networkName,
		Options: types.NetworkCreate{
			CheckDuplicate: true,
			Attachable:     true,
			Driver:         "bridge",
			IPAM: &dockerNetwork.IPAM{
				Driver: "default",
				Config: []dockerNetwork.IPAMConfig{
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

// NewContainerBuilder returns a new *ContainerBuilder.
func (dt *Session) NewContainerBuilder() *ContainerBuilder {
	return &ContainerBuilder{
		clientEnabled: dt.clientEnabled,
		sessionID:     dt.ID,
		ContainerConfig: &container.Config{
			Labels: dt.getLabels(),
		},
		NetworkingConfig: &dockerNetwork.NetworkingConfig{},
		HostConfig:       &container.HostConfig{},
	}
}

func getBasicFilterArgs() filters.Args {
	filterArgs := filters.NewArgs()
	filterArgs.Add("label", fmt.Sprintf("%s=%s", mainLabel, defaultMainLabelValue))

	return filterArgs
}
