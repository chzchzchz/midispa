//go:build !js
// +build !js

package main

import (
	"fmt"
	"net/http"
)

func main() {
	fileServer := http.FileServer(http.Dir("."))
	http.Handle("/", fileServer)
	println("Listening on port 8080...")
	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		fmt.Printf("ERROR: %s\n", err.Error())
	}
}
