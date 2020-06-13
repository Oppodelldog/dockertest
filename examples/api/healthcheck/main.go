package main

import (
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

	r, err := http.DefaultClient.Get(getBaseURL())
	if err != nil {
		os.Exit(1)
	}

	defer func() { _ = r.Body.Close() }()

	if r.StatusCode != http.StatusOK {
		os.Exit(unhealthyExitCode)
	}
}
