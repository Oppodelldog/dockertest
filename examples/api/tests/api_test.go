package tests

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"testing"
)

const defaultBaseAPIURL = "http://localhost:8080"
const someName = "Kermit"

// this test is called in functional-tests, it will fail calling it directly missing necessary setup.
// see examples/api/main.go:60
func TestApi(t *testing.T) {
	t.Logf("running tests against: %s", apiBaseURL())
	ctx := context.Background()
	reqGET, err := http.NewRequestWithContext(ctx, http.MethodGet, apiBaseURL(), nil)
	failOnError(t, err)
	resp, err := http.DefaultClient.Do(reqGET)
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

	var reqPUT *http.Request
	reqPUT, err = http.NewRequestWithContext(ctx, http.MethodPut, fmt.Sprintf("%s/%s", apiBaseURL(), someName), nil)
	failOnError(t, err)

	resp1, err := http.DefaultClient.Do(reqPUT)
	failOnError(t, err)

	defer func() {
		err := resp1.Body.Close()
		if err != nil {
			t.Fatalf("Error closing response body: %v", err)
		}
	}()

	expectedStatusCode := http.StatusOK
	if resp1.StatusCode != expectedStatusCode {
		t.Fatalf("Expected status code to be %v, but got: %v", expectedStatusCode, resp.StatusCode)
	}

	reqGET2, err := http.NewRequestWithContext(ctx, http.MethodGet, apiBaseURL(), nil)
	failOnError(t, err)
	resp2, err := http.DefaultClient.Do(reqGET2)
	failOnError(t, err)

	content, err = io.ReadAll(resp2.Body)
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
