package ports_test

import (
	"github.com/Oppodelldog/dockertest"
	"net"
	"testing"
)

func TestExposeBindPorts(t *testing.T) {
	s, err := dockertest.NewSession()
	failOnError(t, err)

	defer func() { s.Cleanup() }()

	cnt, err := s.NewContainerBuilder().
		Name("test-container").
		Image("busybox").
		Cmd("nc -l -p 15000").
		ExposePort("15000/tcp").
		BindPort("15000/tcp", "15000").
		Build()
	failOnError(t, err)

	err = cnt.Start()
	failOnError(t, err)

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
