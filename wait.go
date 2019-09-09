package dockertest

import (
	"context"
	"fmt"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"reflect"
	"runtime"
	"time"
)

var pollingPause = 1000 * time.Millisecond

type waitForContainerFunc func(inspectResult types.ContainerJSON, inspectError error) bool

func containerIsHealthy(inspectResult types.ContainerJSON, inspectError error) bool {
	return inspectResult.State.Health.Status == "healthy"
}

func containerHasFadeAway(inspectResult types.ContainerJSON, inspectError error) bool {
	return client.IsErrNotFound(inspectError) || !inspectResult.State.Running
}

func waitForContainer(f waitForContainerFunc, ctx context.Context, dockerClient *client.Client, containerID string) bool {
	for {
		select {
		case <-ctx.Done():
			funcName := runtime.FuncForPC(reflect.ValueOf(f).Pointer()).Name()
			fmt.Printf("waiting for '%s' timed out for container %v\n", funcName, containerID)
			return false
		default:
			inspectResult, err := dockerClient.ContainerInspect(ctx, containerID)
			if f(inspectResult, err) {
				return true
			}
			time.Sleep(pollingPause)
		}
	}
}
