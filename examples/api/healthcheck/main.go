package main

import (
	"context"
	"net/http"
	"os"
	"time"
)

const apiBaseURL = "http://localhost:8080"
const clientTimeout = time.Millisecond * 30
const unhealthyExitCode = 2

func getBaseURL() string {
	if baseURLFromEnv, ok := os.LookupEnv("API_BASE_URL"); ok {
		return baseURLFromEnv
	}

	return apiBaseURL
}

func main() {
	http.DefaultClient.Timeout = clientTimeout
	ctx, cancel := context.WithTimeout(context.Background(), clientTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, getBaseURL(), nil)
	if err != nil {
		os.Exit(1)
	}

	resp, err := http.DefaultClient.Do(req)

	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		defer os.Exit(unhealthyExitCode)
	}
}
