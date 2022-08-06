package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Oppodelldog/dockertest"
)

const waitingTimeout = time.Minute

// functional tests for name api.
// nolint:funlen
func main() {
	// the local test dir will help mounting the project into the containers
	projectDir, err := os.Getwd()
	panicOnErr(err)

	// start a new test
	test, err := dockertest.NewSession()
	panicOnErr(err)

	go cancelSessionOnSigTerm(test)

	// cleanup resources from a previous test
	test.CleanupRemains()

	// initialize testResult which is passed into deferred cleanup method
	var testResult = TestResult{ExitCode: -1}
	defer cleanup(test, &testResult)

	// let put test log output into a separate directory
	test.SetLogDir("examples/api/test-logs")

	// since it's a micro-service api test, we need networking facility
	net, err := test.CreateBasicNetwork("test-network").Create()
	panicOnErr(err)

	basicConfiguration := test.NewContainerBuilder().
		Image("golang:1.19.0").
		Connect(net).
		WorkingDir("/app/examples/api").
		Mount(projectDir, "/app")

	// create the API container, the system under test
	api, err := basicConfiguration.NewContainerBuilder().
		Name("api").
		Cmd("go run nameapi/main.go").
		Env("API_BASE_URL", "http://localhost:8080").
		HealthShellCmd("go run healthcheck/main.go").
		Build()
	panicOnErr(err)

	// create the testing container
	tests, err := basicConfiguration.NewContainerBuilder().
		Name("tests").
		Cmd("go test -v tests/api_test.go").
		Link(api, "api", net).
		Env("API_BASE_URL", "http://api:8080").
		Build()
	panicOnErr(err)

	// start api containers
	err = api.Start()
	panicOnErr(err)

	// wait until API is available
	err = <-test.NotifyContainerHealthy(api, waitingTimeout)
	panicOnErr(err)

	// now start the tests
	err = tests.Start()
	panicOnErr(err)

	// wait for tests to finish
	<-test.NotifyContainerExit(tests, waitingTimeout)

	// grab the exit code from the exited container
	testResult.ExitCode, err = tests.ExitCode()
	panicOnErr(err)

	// dump the test output to the log directory

	test.DumpContainerLogsToDir(tests)
}

func cancelSessionOnSigTerm(session *dockertest.Session) {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
	session.Cancel()
}

// it is always a good practise to use defer.
func cleanup(test *dockertest.Session, testResult *TestResult) {
	fmt.Println("CLEANUP-START")
	test.Cleanup()
	fmt.Println("CLEANUP-DONE")

	if r := recover(); r != nil {
		fmt.Printf("ERROR: %v\n", r)
	}

	os.Exit(testResult.ExitCode)
}

// TestResult helps to share the exit code through a defer to the cleanup function.
type TestResult struct {
	ExitCode int
}

func panicOnErr(err error) {
	if err != nil {
		fmt.Println(err)
		panic(err)
	}
}
