package dockertest

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
)

// NewSession creates a new Test and returns a Session instance to work with.
func NewSession() (*Session, error) {
	sessionId := time.Now().Format("20060102150405")
	dockerClient, err := client.NewEnvClient()
	if err != nil {
		return nil, err
	}
	return &Session{
		ctx:                  context.Background(),
		Id:                   sessionId,
		dockerClient:         dockerClient,
		containerStopTimeout: time.Duration(10),
	}, nil
}

//Session is the main object when starting a docker driven container test
type Session struct {
	Id                   string
	logDir               string
	dockerClient         *client.Client
	ctx                  context.Context
	containerDir         string
	containerStopTimeout time.Duration
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

// WaitForContainerToExit waits for the given container to exit. Therefore it waits about 20 seconds
// polling the status of the container. If the operation times out, it will try to kill the container.
func (dt *Session) WaitForContainerToExit(container *Container, timeout time.Duration) chan bool {
	doneCh := make(chan bool)
	go func() {
		ctxTimeout, _ := context.WithTimeout(dt.ctx, timeout)
		if !waitForContainer(containerHasFadeAway, ctxTimeout, dt.dockerClient, container.containerBody.ID) {
			err := dt.dockerClient.ContainerKill(context.Background(), container.containerBody.ID, "kill")
			if err != nil {
				fmt.Println("Error while killing container,", err)
			}
		}
		doneCh <- true
	}()

	return doneCh
}

// WaitForContainerToBeHealthy blocks until the given container reaches healthy state or timeout occurrs.
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
	cleaner := newCleaner(dt)
	cleaner.stopSessionContainers(dt.Id)
	cleaner.removeDockerTestContainers()
	cleaner.cleanupTestNetwork()
}

func (dt *Session) getLabels() map[string]string {
	return map[string]string{
		"docker-dns":         "functional-test",
		"docker-dns-session": fmt.Sprintf("%s", dt.Id),
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
		dt.dumpInspectContainter(c)
	}
}

func (dt *Session) dumpInspectContainter(container *Container) {
	inspectJson, err := dt.dockerClient.ContainerInspect(dt.ctx, container.containerBody.ID)
	if err != nil {
		panicOnError(err)
	}
	b, err := json.Marshal(inspectJson)
	if err != nil {
		fmt.Printf("error serializing inspect json for container '%s': %v\n", container.Name, err)
		return
	}
	fileName := fmt.Sprintf("%s.json", container.Name)
	logFilename := dt.getLogFilename(fileName)
	err = ioutil.WriteFile(logFilename, b, 0655)
	if err != nil {
		fmt.Printf("error writing inspect result to file '%s': %v\n", logFilename, err)
		return
	}
}

// DumpContainerLogs dumps the log of one or multiple containers to the log directory.
func (dt *Session) DumpContainerLogs(container ...*Container) {
	for _, c := range container {
		dt.dumpContainerLog(c)
	}
}

func (dt *Session) dumpContainerLog(container *Container) {
	containerId := container.containerBody.ID
	logReader, err := dt.dockerClient.ContainerLogs(dt.ctx, containerId, types.ContainerLogsOptions{ShowStderr: true, ShowStdout: true})
	if err != nil {
		fmt.Printf("error reading container log for '%s': %v\n", containerId, err)
		return
	}
	log, err := ioutil.ReadAll(logReader)
	if err != nil {
		fmt.Printf("error reading container log stream for '%s': %v\n", containerId, err)
		return
	}
	fileName := fmt.Sprintf("%s.txt", container.Name)
	logFilename := dt.getLogFilename(fileName)
	err = ioutil.WriteFile(logFilename, log, 0655)
	if err != nil {
		fmt.Printf("error writing container log to file '%s': %v\n", logFilename, err)
		return
	}
}

// CreateSimpleNetwork creates a bridged network with the given name, subnet mask and ip range.
func (dt *Session) CreateBasicNetwork(networkName string) *NetworkBuilder {
	cleaner := newCleaner(dt)
	cleaner.cleanupTestNetwork()

	return &NetworkBuilder{
		ctx:          dt.ctx,
		dockerClient: dt.dockerClient,
		Name:         networkName,
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
	cleaner := newCleaner(dt)
	cleaner.cleanupTestNetwork()

	return &NetworkBuilder{
		ctx:          dt.ctx,
		dockerClient: dt.dockerClient,
		Name:         networkName,
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
		ctx:          dt.ctx,
		sessionId:    dt.Id,
		dockerClient: dt.dockerClient,
		ContainerConfig: &container.Config{
			Labels: dt.getLabels(),
		},
		NetworkingConfig: &network.NetworkingConfig{},
		HostConfig:       &container.HostConfig{},
	}
}

func (dt *Session) getLogFilename(filename string) string {
	return path.Join(dt.logDir, filename)
}

func getBasicFilterArgs() filters.Args {
	filterArgs := filters.NewArgs()
	filterArgs.Add("label", "docker-dns=functional-test")
	return filterArgs
}
