package dockertest

import (
	"context"

	"github.com/docker/docker/client"
)

//clientEnabled wraps a docker client and a context for easy passing through compositions.
type clientEnabled struct {
	cancelCtx    context.CancelFunc
	ctx          context.Context
	dockerClient *client.Client
}

func (c clientEnabled) Cancel() {
	c.cancelCtx()
}
