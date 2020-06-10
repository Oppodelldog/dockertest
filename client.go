package dockertest

import (
	"context"

	"github.com/docker/docker/client"
)

//ClientEnabled wraps a docker client and a context for easy passing through compositions.
type ClientEnabled struct {
	cancelCtx    context.CancelFunc
	ctx          context.Context
	dockerClient *client.Client
}

func (c *ClientEnabled) Cancel() {
	c.cancelCtx()
}
