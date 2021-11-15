package ports_test

import (
	"net"
	"testing"
	"time"

	"github.com/Oppodelldog/dockertest"
)

func TestExposeBindPorts(t *testing.T) {
	s, err := dockertest.NewSession()
	failOnError(t, err)

	defer func() { s.Cleanup() }()
	cnt, err := s.NewContainerBuilder().
		Name("test-container").
		Image("busybox").
		Cmd("nc -v -l -p 15000").
		ExposePort("15000/tcp").
		BindPort("15000/tcp", "15000").
		Build()
	failOnError(t, err)

	err = cnt.Start()
	failOnError(t, err)

	failOnError(t, <-s.NotifyContainerLogContains(cnt, time.Second*10, "listening on"))

	time.Sleep(1 * time.Second)

	c, err := net.Dial("tcp", "localhost:15000")
	failOnError(t, err)
	failOnError(t, c.Close())

	cnt.Cancel()
}

func failOnError(t *testing.T, err error) {
	if err != nil {
		t.Fatal(err)
	}
}
