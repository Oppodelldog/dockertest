package tests

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"testing"
)

var apiBaseUrl = "http://localhost:8080"

func init() {
	if baseUrlFromEnv, ok := os.LookupEnv("API_BASE_URL"); ok {
		apiBaseUrl = baseUrlFromEnv
	}
}

const someName = "Kermit"

func TestApi(t *testing.T) {
	t.Logf("running tests against: %s", apiBaseUrl)
	resp, err := http.Get(apiBaseUrl)
	failOnError(t, err)

	content, err := ioutil.ReadAll(resp.Body)
	failOnError(t, err)

	if string(content) != "" {
		t.Fatalf("Did expect and empty result for get in the first call, but got: %v", string(content))
	}

	req, err := http.NewRequest("PUT", fmt.Sprintf("%s/%s", apiBaseUrl, someName), nil)
	resp, err = http.DefaultClient.Do(req)
	failOnError(t, err)

	expectedStatusCode := http.StatusOK
	if resp.StatusCode != expectedStatusCode {
		t.Fatalf("Expected status code to be %v, but got: %v", expectedStatusCode, resp.StatusCode)
	}

	resp, err = http.Get(apiBaseUrl)
	failOnError(t, err)

	content, err = ioutil.ReadAll(resp.Body)
	failOnError(t, err)

	if string(content) != someName {
		t.Fatalf("Did expect to get '%s', but got: %v", someName, string(content))
	}
}

func failOnError(t *testing.T, err error) {
	if err != nil {
		t.Fatalf("Did not expect and error, but got: %v ", err)
	}
}
