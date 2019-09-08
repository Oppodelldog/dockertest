package dockertest

import (
	"context"
	"fmt"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"sync"
	"time"
)

func newCleaner(dt *DockerTest) Cleaner {
	return Cleaner{dockerClient: dt.dockerClient, ctx: dt.ctx, containerStopTimeout: time.Second * 10}
}

type Cleaner struct {
	ctx                  context.Context
	dockerClient         *client.Client
	containerStopTimeout time.Duration
}

func (c *Cleaner) cleanupTestNetwork() {
	res, err := c.dockerClient.NetworkList(c.ctx, types.NetworkListOptions{Filters: getBasicFilterArgs()})
	panicOnError(err)
	for _, networkResource := range res {
		err := c.dockerClient.NetworkRemove(c.ctx, networkResource.ID)
		if err != nil {
			fmt.Printf("could not remove network: %v\n", err)
		}
	}
}

func (c *Cleaner) removeDockerTestContainers() {
	removeContainers := &sync.WaitGroup{}
	args := getBasicFilterArgs()
	args.Add("status", "exited")
	exitedContainers, err := c.dockerClient.ContainerList(c.ctx, types.ContainerListOptions{All: true, Filters: args})
	if err == nil {
		removeContainers.Add(len(exitedContainers))
		for _, testContainer := range exitedContainers {
			go c.removeContainer(testContainer.ID, removeContainers)
		}
	} else {
		fmt.Printf("error finding dockertest containers: %v\n", err)
	}
	removeContainers.Wait()
}

func (c *Cleaner) stopSessionContainers(sessionId int) {
	shutDownContainers := &sync.WaitGroup{}
	args := getBasicFilterArgs()
	args.Add("docker-dns-session", fmt.Sprintf("%v", sessionId))
	args.Add("status", "running")
	containers, err := c.dockerClient.ContainerList(c.ctx, types.ContainerListOptions{All: true, Filters: args})
	if err == nil {
		shutDownContainers.Add(len(containers))
		for _, testContainer := range containers {
			go c.shutDownContainer(testContainer.ID, shutDownContainers)
		}
	} else {
		fmt.Printf("error finding session containers: %v\n", err)
	}
	shutDownContainers.Wait()

}

func (c *Cleaner) removeContainer(containerID string, wg *sync.WaitGroup) {
	_ = c.dockerClient.ContainerRemove(c.ctx, containerID, types.ContainerRemoveOptions{RemoveVolumes: true, RemoveLinks: false, Force: true})
	wg.Done()
}

func (c *Cleaner) shutDownContainer(containerID string, wg *sync.WaitGroup) {
	stopTimeout := c.containerStopTimeout
	_ = c.dockerClient.ContainerStop(c.ctx, containerID, &stopTimeout)

	waitContainerToFadeAway(c.ctx, c.dockerClient, containerID)
	wg.Done()
}
