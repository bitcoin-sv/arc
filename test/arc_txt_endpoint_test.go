package main

import (
	"bytes"
	"net/http"
	"testing"
)

func TestHttpPost(t *testing.T) {
	// The URL to send the POST request to.
	url := "http://arc:9090/arc/v1/txs"

	// The request body data.
	data := []byte("{}")

	// Create a new request using http.
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(data))

	// If there is an error while creating the request, fail the test.
	if err != nil {
		t.Fatalf("Error creating HTTP request: %s", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "text/plain")

	// Send the request using http.Client.
	client := &http.Client{}
	resp, err := client.Do(req)

	// If there is an error while sending the request, fail the test.
	if err != nil {
		t.Fatalf("Error sending HTTP request: %s", err)
	}

	defer resp.Body.Close()

	// Check the HTTP status code.
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status OK, got: %s", resp.Status)
	}
}
