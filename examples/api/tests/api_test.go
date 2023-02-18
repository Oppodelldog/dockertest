package tests

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"testing"
)

const defaultBaseAPIURL = "http://localhost:8080"
const someName = "Kermit"

func TestApi(t *testing.T) {
	t.Logf("running tests against: %s", apiBaseURL())
	resp, err := http.Get(apiBaseURL())
	failOnError(t, err)

	defer func() {
		err := resp.Body.Close()
		if err != nil {
			t.Fatalf("Error closing response body: %v", err)
		}
	}()

	content, err := io.ReadAll(resp.Body)
	failOnError(t, err)

	if string(content) != "" {
		t.Fatalf("Did expect and empty result for get in the first call, but got: %v", string(content))
	}

	var req *http.Request
	req, err = http.NewRequest("PUT", fmt.Sprintf("%s/%s", apiBaseURL(), someName), nil)
	failOnError(t, err)

	resp, err = http.DefaultClient.Do(req)
	failOnError(t, err)

	defer func() {
		err := resp.Body.Close()
		if err != nil {
			t.Fatalf("Error closing response body: %v", err)
		}
	}()

	expectedStatusCode := http.StatusOK
	if resp.StatusCode != expectedStatusCode {
		t.Fatalf("Expected status code to be %v, but got: %v", expectedStatusCode, resp.StatusCode)
	}

	resp, err = http.Get(apiBaseURL())
	failOnError(t, err)

	content, err = io.ReadAll(resp.Body)
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

func apiBaseURL() string {
	if baseURLFromEnv, ok := os.LookupEnv("API_BASE_URL"); ok {
		return baseURLFromEnv
	}

	return defaultBaseAPIURL
}
