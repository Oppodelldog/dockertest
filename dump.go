package dockertest

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/docker/docker/pkg/stdcopy"
	"io"
	"io/ioutil"
	"path"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
)

const dumpFileMask = 0655

var ErrReadingContainerLog = errors.New("error reading container log")

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
	log, err := getContainerLog(ctx, dockerClient, container)
	if err != nil {
		fmt.Printf("error reading logs from container '%v: %v\n", container.Name, err)
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

func getContainerLog(ctx context.Context, dockerClient *client.Client, container *Container) ([]byte, error) {
	containerID := container.containerBody.ID

	logReader, err := dockerClient.ContainerLogs(
		ctx,
		containerID,
		types.ContainerLogsOptions{ShowStderr: true, ShowStdout: true},
	)
	if err != nil {
		return nil, fmt.Errorf("%w for '%s': %v", ErrReadingContainerLog, containerID, err)
	}

	log := bytes.NewBufferString("")
	_, err = stdcopy.StdCopy(log, log, logReader)
	if err != nil {
		return nil, fmt.Errorf("%w stream for '%s': %v", ErrReadingContainerLog, containerID, err)
	}

	return log.Bytes(), nil
}

func writeLog(w io.Writer, c *Container, log []byte) {
	writes := []func() (n int, err error){
		func() (n int, err error) {
			return w.Write([]byte(fmt.Sprintf("\n------ Container Log '%s':\n", c.Name)))
		},
		func() (n int, err error) { return w.Write(log) },
		func() (n int, err error) {
			return w.Write([]byte(fmt.Sprintf("\n------ End of '%s' container log.\n\n", c.Name)))
		},
	}

	for _, write := range writes {
		_, err := write()
		if err != nil {
			fmt.Printf("error writing container '%s' log: %v", c.Name, err)
			return
		}
	}
}
