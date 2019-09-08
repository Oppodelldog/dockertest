package main

import (
	"fmt"
	"net/http"
	"strings"
)

var storage []string

func main() {
	err := http.ListenAndServe("0.0.0.0:8080", http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			switch r.Method {
			case http.MethodGet:
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(strings.Join(storage, ",")))
			case http.MethodPut:
				newName := strings.TrimPrefix(r.RequestURI, "/")
				storage = append(storage, newName)
				w.WriteHeader(http.StatusOK)
			}
		},
	))

	if err != nil {
		fmt.Println(err)
	}
}
