package dockertest

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"reflect"
	"runtime"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
)

var ErrClosedWithoutFinding = errors.New("log stream closed without finding")

var pollingPause = 1000 * time.Millisecond

type waitForContainerFunc func(inspectResult types.ContainerJSON, inspectError error) bool

func containerIsHealthy(inspectResult types.ContainerJSON, _ error) bool {
	return inspectResult.State.Health.Status == "healthy"
}

func containerHasFadeAway(inspectResult types.ContainerJSON, inspectError error) bool {
	return client.IsErrNotFound(inspectError) || !inspectResult.State.Running
}

func waitForContainer(
	ctx context.Context,
	f waitForContainerFunc,
	dockerClient *client.Client,
	containerID string,
) bool {
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

func waitForContainerLog(ctx context.Context, search string, dockerClient *client.Client, containerID string) error {
	var logOpts = types.ContainerLogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     true,
	}

	reader, err := dockerClient.ContainerLogs(ctx, containerID, logOpts)
	if err != nil {
		return err
	}

	defer func() {
		_ = reader.Close()
	}()

	var (
		buffer  = strings.Builder{}
		scanner = bufio.NewScanner(reader)
	)

	for scanner.Scan() {
		if strings.Contains(scanner.Text(), search) {
			return nil
		} else {
			buffer.WriteString(scanner.Text() + "\n")
		}
	}

	return fmt.Errorf("%w '%s' (output: %s)", ErrClosedWithoutFinding, search, buffer.String())
}
