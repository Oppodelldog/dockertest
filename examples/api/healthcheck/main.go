package main

import (
	"net/http"
	"os"
	"time"
)

var apiBaseUrl = "http://localhost:8080"

func init() {
	if baseUrlFromEnv, ok := os.LookupEnv("API_BASE_URL"); ok {
		apiBaseUrl = baseUrlFromEnv
	}
}

func main() {
	http.DefaultClient.Timeout = time.Millisecond * 30
	r, err := http.DefaultClient.Get(apiBaseUrl)
	if err != nil {
		os.Exit(1)
	}
	if r.StatusCode != 200 {
		os.Exit(2)
	}
}
