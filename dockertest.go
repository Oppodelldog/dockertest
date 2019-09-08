package dockertest

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strings"
	"syscall"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/strslice"
	"github.com/docker/docker/client"
)

// New creates a new Test and returns a DockerTest instance to work with.
func New() (*DockerTest, error) {
	sessionId := time.Now().Format("20060102150405")
	dockerClient, err := client.NewEnvClient()
	if err != nil {
		return nil, err
	}
	return &DockerTest{
		ctx:                  context.Background(),
		sessionId:            sessionId,
		dockerClient:         dockerClient,
		containerStopTimeout: time.Duration(10),
	}, nil
}

//DockerTest is the main object when starting a docker driven container test
type DockerTest struct {
	logDir               string
	sessionId            string
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
func (dt *DockerTest) SetLogDir(logDir string) {
	err := os.MkdirAll(logDir, 0777)
	panicOnError(err)
	dt.logDir = logDir
}

// WaitForContainerToExit waits for the given container to exit. Therefore it waits about 20 seconds
// polling the status of the container. If the operation times out, it will try to kill the container.
// Arfter that it sends a SIGTERM to the currently running process.
func (dt *DockerTest) WaitForContainerToExit(container *Container) {
	go func() {
		if !waitContainerToFadeAway(dt.ctx, dt.dockerClient, container.containerBody.ID) {
			err := dt.dockerClient.ContainerKill(context.Background(), container.containerBody.ID, "kill")
			if err != nil {
				fmt.Println("Error while killing container,", err)
			}
		}
		_ = syscall.Kill(syscall.Getpid(), syscall.SIGTERM)

	}()
}

func waitContainerToFadeAway(ctx context.Context, dockerClient *client.Client, containerID string) bool {
	var i = 0
	for {
		i++
		insp, err := dockerClient.ContainerInspect(ctx, containerID)

		insp = insp
		if client.IsErrNotFound(err) || !insp.State.Running {
			return true
		}

		time.Sleep(1 * time.Second)
		if i == 20 {
			fmt.Println("waiting for tests to finish timed out ", containerID)
			return false
		}
	}
}

// Cleanup removes all resources (like containers/networks) used for the test
func (dt *DockerTest) Cleanup() {
	cleaner := newCleaner(dt)
	cleaner.stopSessionContainers(dt.sessionId)
	cleaner.removeDockerTestContainers()
	cleaner.cleanupTestNetwork()
}

func (dt *DockerTest) getLabels() map[string]string {
	return map[string]string{
		"docker-dns":         "functional-test",
		"docker-dns-session": fmt.Sprintf("%s", dt.sessionId),
	}
}

// StartContainer starts one or multiple given containers.
// If some containers return error while starting the last error will be returned.
func (dt *DockerTest) StartContainer(container ...*Container) error {
	var err error
	for _, c := range container {
		errStart := c.StartContainer()
		if errStart != nil {
			err = errStart
		}
	}

	return err
}

// DumpInspect dumps an json file with the content of "docker inspect" into the log directory.
func (dt *DockerTest) DumpInspect(container ...*Container) {
	for _, c := range container {
		dt.dumpInspectContainter(c)
	}
}

func (dt *DockerTest) dumpInspectContainter(container *Container) {
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
func (dt *DockerTest) DumpContainerLogs(container ...*Container) {
	for _, c := range container {
		dt.dumpContainerLog(c)
	}
}

func (dt *DockerTest) dumpContainerLog(container *Container) {
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
func (dt *DockerTest) CreateSimpleNetwork(networkName, subNet, ipRange string) *NetworkBuilder {
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

func (dt *DockerTest) createBaseContainerStructs(cmd string, image string) (*container.Config, *container.HostConfig, *network.NetworkingConfig) {
	containerConfig := &container.Config{
		Env:    []string{},
		Image:  image,
		Cmd:    strslice.StrSlice(strings.Split(cmd, " ")),
		Labels: dt.getLabels(),
	}

	hostConfig := &container.HostConfig{
		AutoRemove: false,
	}

	networkConfig := &network.NetworkingConfig{}

	return containerConfig, hostConfig, networkConfig
}

// NewContainer creates a new container builder prepared with the given inputs.
func (dt *DockerTest) NewContainer(containerName, image, cmd string) *ContainerBuilder {
	containerConfig, hostConfig, networkConfig := dt.createBaseContainerStructs(cmd, image)
	return &ContainerBuilder{
		ContainerConfig:  containerConfig,
		HostConfig:       hostConfig,
		NetworkingConfig: networkConfig,
		ContainerName:    fmt.Sprintf("%s-%s", containerName, dt.sessionId),
		originalName:     containerName,
		ctx:              dt.ctx,
		dockerClient:     dt.dockerClient,
	}
}

func (dt *DockerTest) getLogFilename(filename string) string {
	return path.Join(dt.logDir, filename)
}

func getBasicFilterArgs() filters.Args {
	filterArgs := filters.NewArgs()
	filterArgs.Add("label", "docker-dns=functional-test")
	return filterArgs
}
