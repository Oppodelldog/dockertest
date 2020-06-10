package dockertest

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
)

func newCleaner(ctx context.Context, dt *Session) cleaner {
	return cleaner{dockerClient: dt.dockerClient, ctx: ctx, containerStopTimeout: cleanerTimeout}
}

type cleaner struct {
	ctx                  context.Context
	dockerClient         *client.Client
	containerStopTimeout time.Duration
}

func (c *cleaner) cleanupTestNetwork() {
	res, err := c.dockerClient.NetworkList(c.ctx, types.NetworkListOptions{Filters: getBasicFilterArgs()})
	panicOnError(err)

	for _, networkResource := range res {
		err := c.dockerClient.NetworkRemove(c.ctx, networkResource.ID)
		if err != nil {
			fmt.Printf("could not remove network: %v\n", err)
		}
	}
}

func (c *cleaner) removeDockerTestContainers(sessionID string) {
	removeContainers := &sync.WaitGroup{}
	args := getBasicFilterArgs()
	args.Add("label", fmt.Sprintf("docker-dns-session=%s", sessionID))

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

func (c *cleaner) stopSessionContainers(sessionID string) {
	shutDownContainers := &sync.WaitGroup{}
	args := getBasicFilterArgs()
	args.Add("label", fmt.Sprintf("docker-dns-session=%s", sessionID))
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

func (c *cleaner) removeContainer(containerID string, wg *sync.WaitGroup) {
	_ = c.dockerClient.ContainerRemove(c.ctx,
		containerID,
		types.ContainerRemoveOptions{RemoveVolumes: true, RemoveLinks: false, Force: true},
	)

	wg.Done()
}

func (c *cleaner) shutDownContainer(containerID string, wg *sync.WaitGroup) {
	stopTimeout := c.containerStopTimeout
	_ = c.dockerClient.ContainerStop(c.ctx, containerID, &stopTimeout)

	waitForContainer(c.ctx, containerHasFadeAway, c.dockerClient, containerID)
	wg.Done()
}
