package dockertest

import (
	"context"
	"fmt"
	"github.com/docker/docker/client"
	"time"
)

func waitContainerToBeHealthy(ctx context.Context, dockerClient *client.Client, containerID string) bool {
	var i = 0
	for {
		i++
		insp, err := dockerClient.ContainerInspect(ctx, containerID)
		if err != nil {
			time.Sleep(1 * time.Second)
			continue
		}

		insp = insp
		if insp.State.Health.Status == "healthy" {
			return true
		}

		time.Sleep(1 * time.Second)
		if i == 20 {
			fmt.Println("waiting for tests to be healthy timed out ", containerID)
			return false
		}
	}
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
