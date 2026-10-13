package main

import (
	"fmt"
	"net/http"
	"os"
	"time"
)

func check(base string) error {
	client := http.Client{Timeout: 3 * time.Second}
	response, err := client.Get(base + "/api/v1/health/ready")
	if err != nil {
		return fmt.Errorf("API is unavailable")
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("API is not ready")
	}
	return nil
}
func main() {
	if err := check("http://127.0.0.1:8080"); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
