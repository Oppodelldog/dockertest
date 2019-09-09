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

const cleanerTimeout = 10 * time.Second
const mainLabel = "dockertest"
const mainLabelValue = "dockertest"
const sessionLabel = "docker-dns-session"

// NewSession creates a new Test and returns a Session instance to work with.
func NewSession() (*Session, error) {
	sessionId := time.Now().Format("20060102150405")
	dockerClient, err := client.NewEnvClient()
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &Session{

		Id: sessionId,
		ClientEnabled: ClientEnabled{
			cancelCtx:    cancel,
			ctx:          ctx,
			dockerClient: dockerClient,
		},
		containerStopTimeout: time.Duration(10),
	}, nil
}

//Session is the main object when starting a docker driven container test
type Session struct {
	Id                   string
	logDir               string
	containerDir         string
	containerStopTimeout time.Duration
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
		ctxTimeout, _ := context.WithTimeout(dt.ctx, timeout)
		if !waitForContainer(containerHasFadeAway, ctxTimeout, dt.dockerClient, container.containerBody.ID) {
			err := dt.dockerClient.ContainerKill(context.Background(), container.containerBody.ID, "kill")
			if err != nil {
				fmt.Println("Error while killing container,", err)
			}
		}
		exitedCh <- true
	}()

	return exitedCh
}

// WaitForContainerToBeHealthy returns a channel that blocks until the given container reaches healthy state or timeout occurrs.
func (dt *Session) WaitForContainerToBeHealthy(container *Container, timeout time.Duration) chan error {
	healthErr := make(chan error)
	go func() {
		ctxTimeout, _ := context.WithTimeout(dt.ctx, timeout)
		if !waitForContainer(containerIsHealthy, ctxTimeout, dt.dockerClient, container.containerBody.ID) {
			healthErr <- errors.New("timeout - container is not healthy")
		}
		healthErr <- nil
	}()

	return healthErr
}

// Cleanup removes all resources (like containers/networks) used for the test
func (dt *Session) Cleanup() {
	cleaner := newCleaner(dt, cleanerTimeout)
	cleaner.stopSessionContainers(dt.Id)
	cleaner.removeDockerTestContainers(dt.Id)
	cleaner.cleanupTestNetwork()
}

func (dt *Session) getLabels() map[string]string {
	return map[string]string{
		mainLabel:    mainLabelValue,
		sessionLabel: fmt.Sprintf("%s", dt.Id),
	}
}

// Start starts one or multiple given containers.
// If some containers return error while starting the last error will be returned.
func (dt *Session) StartContainer(container ...*Container) error {
	var err error
	for _, c := range container {
		errStart := c.Start()
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
	cleaner := newCleaner(dt, cleanerTimeout)
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
	cleaner := newCleaner(dt, cleanerTimeout)
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
		sessionId:     dt.Id,
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
