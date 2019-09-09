package main

import (
	"fmt"
	"github.com/Oppodelldog/dockertest"
	"os"
	"runtime/debug"
	"time"
)

const waitingTimeout = time.Minute

// functional tests for name api
func main() {
	// the local test dir will help mounting the project into the containers
	projectDir, err := os.Getwd()
	panicOnErr(err)

	// start a new test
	test, err := dockertest.New()
	panicOnErr(err)

	// initialize testResult which is passed into deferred cleanup method
	var testResult = &TestResult{ExitCode: -1}
	defer cleanup(test, testResult)

	// let put test log output into a separate directory
	test.SetLogDir("examples/api/test-logs")

	// since it's a micro-service api test, we need networking facility
	net, err := test.CreateBasicNetwork("test-network").Create()
	panicOnErr(err)

	basicConfiguration := test.NewContainerBuilder().
		Image("golang:1.13.0").
		Connect(net).
		WorkingDir("/app/examples/api").
		Mount(projectDir, "/app")

	// create the API container, the micro-service under test
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
	err = <-test.WaitForContainerToBeHealthy(api, waitingTimeout)
	panicOnErr(err)

	// now start the tests
	err = tests.Start()
	panicOnErr(err)

	// wait for tests to finish
	<-test.WaitForContainerToExit(tests, waitingTimeout)

	// grab the exit code from the exited container
	testResult.ExitCode, err = tests.ExitCode()
	panicOnErr(err)

	// dump the test output to the log directory
	test.DumpContainerLogs(tests)
}

// it is always a good practise to use defer.
func cleanup(test *dockertest.Session, testResult *TestResult) {
	test.Cleanup()
	if r := recover(); r != nil {
		fmt.Println("ERROR: %v", string(debug.Stack()))
	}
	os.Exit(testResult.ExitCode)
}

type TestResult struct {
	ExitCode int
}

func panicOnErr(err error) {
	if err != nil {
		panic(err)
	}
}
