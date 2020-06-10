package dockertest

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
)

const dumpFileMask = 0655

func dumpInspectContainter(ctx context.Context, dockerClient *client.Client, container *Container, logDir string) {
	inspectJSON, err := dockerClient.ContainerInspect(ctx, container.containerBody.ID)
	if err != nil {
		panicOnError(err)
	}

	b, err := json.Marshal(inspectJSON)
	if err != nil {
		fmt.Printf("error serializing inspect json for container '%s': %v\n", container.Name, err)
		return
	}

	fileName := fmt.Sprintf("%s.json", container.Name)
	logFilename := path.Join(logDir, fileName)

	err = ioutil.WriteFile(logFilename, b, dumpFileMask)
	if err != nil {
		fmt.Printf("error writing inspect result to file '%s': %v\n", logFilename, err)
		return
	}
}

func dumpContainerLog(ctx context.Context, dockerClient *client.Client, container *Container, logDir string) {
	containerID := container.containerBody.ID

	logReader, err := dockerClient.ContainerLogs(
		ctx,
		containerID,
		types.ContainerLogsOptions{ShowStderr: true, ShowStdout: true},
	)
	if err != nil {
		fmt.Printf("error reading container log for '%s': %v\n", containerID, err)
		return
	}

	log, err := ioutil.ReadAll(logReader)
	if err != nil {
		fmt.Printf("error reading container log stream for '%s': %v\n", containerID, err)
		return
	}

	fileName := fmt.Sprintf("%s.txt", container.Name)
	logFilename := path.Join(logDir, fileName)

	err = ioutil.WriteFile(logFilename, log, dumpFileMask)
	if err != nil {
		fmt.Printf("error writing container log to file '%s': %v\n", logFilename, err)
		return
	}
}
