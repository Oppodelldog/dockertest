package main

import (
	"github.com/Oppodelldog/dockertest"
	"os"
	"time"
)

const image = "golang:1.13.0"
const containerWd = "/app/examples/api"
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

	// create the API container, the micro-service under test
	api, err := test.NewContainer("api", image, "go run nameapi/main.go").
		ConnectToNetwork(net).
		SetWorkingDir(containerWd).
		Mount(projectDir, "/app").
		SetEnv("API_BASE_URL", "http://localhost:8080").
		SetHealthShellCmd("go run healthcheck/main.go").
		CreateContainer()
	panicOnErr(err)

	// create the testing container
	tests, err := test.NewContainer("tests", image, "go test -v examples/api/tests/api_test.go").
		ConnectToNetwork(net).
		SetWorkingDir("/app").
		Mount(projectDir, "/app").
		Link(api, "api", net).
		SetEnv("API_BASE_URL", "http://api:8080").
		CreateContainer()
	panicOnErr(err)

	// start api containers
	err = api.StartContainer()
	panicOnErr(err)

	// wait until API is available
	err = <-test.WaitForContainerToBeHealthy(api, waitingTimeout)
	panicOnErr(err)

	// now start the tests
	err = tests.StartContainer()
	panicOnErr(err)

	// wait for tests to finish
	<-test.WaitForContainerToExit(tests, waitingTimeout)

	// grab the exit code from the exited container
	testResult.ExitCode, err = tests.GetExitCode()
	panicOnErr(err)

	// dump the test output to the log directory
	test.DumpContainerLogs(tests)
}

// it is always a good practise to use defer.
func cleanup(test *dockertest.DockerTest, testResult *TestResult) {
	test.Cleanup()
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
