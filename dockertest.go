package dockertest

import (
	"context"
	"fmt"
	"io/ioutil"
	"math/rand"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/strslice"
	"github.com/docker/docker/client"
)

func New() (*DockerTest, error) {
	sessionId := rand.New(rand.NewSource(int64(time.Now().Nanosecond()))).Int()
	dockerClient, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		return nil, err
	}
	return &DockerTest{
		sessionId:            sessionId,
		dockerClient:         dockerClient,
		ctx:                  context.Background(),
		containerStopTimeout: time.Duration(10),
	}, nil
}

type Net struct {
	NetworkID   string
	NetworkName string
}

type DockerTest struct {
	sessionId            int
	dockerClient         *client.Client
	ctx                  context.Context
	containerDir         string
	network              *Net
	containerStopTimeout time.Duration
}

func panicOnError(err error) {
	if err != nil {
		panic(err)
	}
}

func (dt *DockerTest) WaitForContainerToExit(container *Container) {
	go func() {
		if !dt.waitContainerToFadeAway(container.containerBody.ID) {
			err := dt.dockerClient.ContainerKill(context.Background(), container.containerBody.ID, "kill")
			if err != nil {
				fmt.Println("Error while killing container,", err)
			}
			_ = syscall.Kill(syscall.Getpid(), syscall.SIGTERM)
		}
	}()
}

func (dt *DockerTest) waitContainerToFadeAway(containerID string) bool {
	var i = 0
	for {
		i++
		_, err := dt.dockerClient.ContainerInspect(dt.ctx, containerID)

		if client.IsErrNotFound(err) {
			return true
		}

		time.Sleep(1 * time.Second)
		if i == 20 {
			fmt.Println("waiting for tests to finish timed out")
			return false
		}
	}
}

func (dt *DockerTest) Cleanup() {
	shutDownContainers := &sync.WaitGroup{}
	containers, err := dt.dockerClient.ContainerList(dt.ctx, types.ContainerListOptions{Filters: getFilterArgs()})
	if err == nil {
		shutDownContainers.Add(len(containers))
		for _, testContainer := range containers {
			go dt.shutDownContainer(testContainer.ID, shutDownContainers)
		}
	} else {
		fmt.Printf("error finding test containers: %v\n", err)
	}
	shutDownContainers.Wait()
	dt.CleanupTestNetwork()
}

type Container struct {
	containerBody container.ContainerCreateCreatedBody
	StartOptions  types.ContainerStartOptions
	ctx           context.Context
	dockerClient  *client.Client
}

func (c *Container) StartContainer() error {
	return c.dockerClient.ContainerStart(c.ctx, c.containerBody.ID, c.StartOptions)
}

func (dt *DockerTest) shutDownContainer(containerID string, wg *sync.WaitGroup) {
	stopTimeout := dt.containerStopTimeout
	_ = dt.dockerClient.ContainerStop(dt.ctx, containerID, &stopTimeout)

	dt.waitContainerToFadeAway(containerID)
	wg.Done()
}

func getLabels() map[string]string {
	return map[string]string{"docker-dns": "functional-test"}
}

func getFilterArgs() filters.Args {
	filterArgs := filters.NewArgs()
	filterArgs.Add("label", "docker-dns=functional-test")
	return filterArgs
}

func (dt *DockerTest) CleanupTestNetwork() {
	res, err := dt.dockerClient.NetworkList(dt.ctx, types.NetworkListOptions{Filters: getFilterArgs()})
	panicOnError(err)
	for _, networkResource := range res {
		err := dt.dockerClient.NetworkRemove(dt.ctx, networkResource.ID)
		if err != nil {
			fmt.Printf("could not remove network: %v\n", err)
		}
	}
}

func (dt *DockerTest) DumpContainerLogs(ctx context.Context, container *Container) {
	containerId := container.containerBody.ID
	fmt.Printf("Container log: %s\n", containerId)
	logReader, err := dt.dockerClient.ContainerLogs(ctx, containerId, types.ContainerLogsOptions{ShowStderr: true, ShowStdout: true})
	if err != nil {
		fmt.Printf("error reading container log for '%s': %v\n", containerId, err)
		return
	}

	log, err := ioutil.ReadAll(logReader)
	if err != nil {
		fmt.Printf("error reading container log stream for '%s': %v\n", containerId, err)
		return
	}

	fmt.Println(string(log))
}

func (dt *DockerTest) CreateNetwork(networkName, subNet, ipRange string) {
	dt.CleanupTestNetwork()

	options := types.NetworkCreate{
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
		Labels: getLabels(),
	}

	resp, err := dt.dockerClient.NetworkCreate(dt.ctx, networkName, options)
	panicOnError(err)

	dt.network = &Net{resp.ID, networkName}
}

func (dt *DockerTest) CreateBaseContainerStructs(cmd string, image string) (*container.Config, *container.HostConfig, *network.NetworkingConfig) {
	containerConfig := &container.Config{
		Env:    []string{},
		Image:  image,
		Cmd:    strslice.StrSlice(strings.Split(cmd, " ")),
		Labels: getLabels(),
	}

	hostConfig := &container.HostConfig{
		AutoRemove:  false,
		NetworkMode: container.NetworkMode(dt.network.NetworkName),
	}

	networkConfig := &network.NetworkingConfig{
		EndpointsConfig: map[string]*network.EndpointSettings{
			dt.network.NetworkName: {
				NetworkID: dt.network.NetworkID,
			},
		},
	}

	return containerConfig, hostConfig, networkConfig
}

type ContainerBuilder struct {
	ContainerConfig  *container.Config
	HostConfig       *container.HostConfig
	NetworkingConfig *network.NetworkingConfig
	dockerClient     *client.Client
	ContainerName    string
	ctx              context.Context
}

func (b *ContainerBuilder) CreateContainer() (*Container, error) {
	containerBody, err := b.dockerClient.ContainerCreate(b.ctx, b.ContainerConfig, b.HostConfig, b.NetworkingConfig, b.ContainerName)
	if err != nil {
		return nil, err
	}
	return &Container{
		containerBody: containerBody,
		ctx:           b.ctx,
		dockerClient:  b.dockerClient,
	}, nil

}

func (dt *DockerTest) NewContainer(containerName, image, cmd string) *ContainerBuilder {
	containerConfig, hostConfig, networkConfig := dt.CreateBaseContainerStructs(cmd, image)
	return &ContainerBuilder{
		ContainerConfig:  containerConfig,
		HostConfig:       hostConfig,
		NetworkingConfig: networkConfig,
		ContainerName:    fmt.Sprintf("%v-%v", containerName, dt.sessionId),
		ctx:              dt.ctx,
		dockerClient:     dt.dockerClient,
	}
}
