package dockertest

import (
	"context"
	"fmt"
	"github.com/docker/docker/api/types/container"
	"sync"
	"time"

	"github.com/docker/docker/api/types/filters"

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

func (c cleaner) cleanupTestNetwork() {
	removeNetworks(c.ctx, getBasicFilterArgs(), c.dockerClient)
}

func (c cleaner) removeDockerTestContainers(sessionID string) {
	args := filterSessionID(getBasicFilterArgs(), sessionID)

	removeContainers(c.ctx, args, c.dockerClient)
}

func (c cleaner) stopSessionContainers(sessionID string) {
	filterArgs := getBasicFilterArgs()
	filterArgs = filterSessionID(filterArgs, sessionID)
	filterArgs = filterContainerRunning(filterArgs)

	stopContainers(c.ctx, filterArgs, c.dockerClient, c.containerStopTimeout)
}

func newRemainsCleaner(ctx context.Context, dc *client.Client) remainsCleaner {
	return remainsCleaner{dockerClient: dc, ctx: ctx, containerStopTimeout: cleanerTimeout}
}

type remainsCleaner struct {
	ctx                  context.Context
	dockerClient         *client.Client
	containerStopTimeout time.Duration
}

func (c remainsCleaner) cleanupTestNetwork() {
	removeNetworks(c.ctx, getBasicFilterArgs(), c.dockerClient)
}

func (c remainsCleaner) removeDockerTestContainers() {
	removeContainers(c.ctx, getBasicFilterArgs(), c.dockerClient)
}

func (c remainsCleaner) stopContainers() {
	stopContainers(c.ctx, getBasicFilterArgs(), c.dockerClient, c.containerStopTimeout)
}

func filterSessionID(args filters.Args, sessionID string) filters.Args {
	args.Add("label", fmt.Sprintf("%s=%s", sessionLabel, sessionID))

	return args
}

func filterContainerRunning(args filters.Args) filters.Args {
	args.Add("status", "running")

	return args
}

func removeNetworks(ctx context.Context, filterArgs filters.Args, dc *client.Client) {
	res, err := dc.NetworkList(ctx, types.NetworkListOptions{Filters: filterArgs})
	panicOnError(err)

	for _, networkResource := range res {
		removeNetwork(ctx, networkResource.ID, dc)
	}
}

func removeContainers(ctx context.Context, filterArgs filters.Args, dc *client.Client) {
	exitedContainers, err := dc.ContainerList(ctx, types.ContainerListOptions{All: true, Filters: filterArgs})
	if err == nil {
		wg := &sync.WaitGroup{}
		wg.Add(len(exitedContainers))

		for _, testContainer := range exitedContainers {
			go func(ID string) {
				removeContainer(ctx, ID, dc)
				wg.Done()
			}(testContainer.ID)
		}

		wg.Wait()
	} else {
		fmt.Printf("error finding dockertest containers: %v\n", err)
	}
}

func stopContainers(ctx context.Context, filterArgs filters.Args, dc *client.Client, timeout time.Duration) {
	containers, err := dc.ContainerList(ctx, types.ContainerListOptions{All: true, Filters: filterArgs})
	if err == nil {
		wg := &sync.WaitGroup{}
		wg.Add(len(containers))

		for _, testContainer := range containers {
			go func(id string) {
				go shutDownContainer(ctx, id, dc, int(timeout))
				wg.Done()
			}(testContainer.ID)
		}

		wg.Wait()
	} else {
		fmt.Printf("error finding session containers: %v\n", err)
	}
}

func shutDownContainer(ctx context.Context, containerID string, dc *client.Client, timeout int) {
	_ = dc.ContainerStop(ctx, containerID, container.StopOptions{
		Timeout: &timeout,
	})
	waitForContainer(ctx, containerHasFadeAway, dc, containerID)
}

func removeContainer(ctx context.Context, containerID string, dc *client.Client) {
	_ = dc.ContainerRemove(ctx,
		containerID,
		types.ContainerRemoveOptions{RemoveVolumes: true, RemoveLinks: false, Force: true},
	)
}

func removeNetwork(ctx context.Context, networkID string, dc *client.Client) {
	err := dc.NetworkRemove(ctx, networkID)
	if err != nil {
		fmt.Printf("could not remove Network: %v\n", err)
	}
}
