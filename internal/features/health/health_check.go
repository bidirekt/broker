package health

import (
	"fmt"
	"net"
	"net/http"
	"time"
)

const checkTimeout = 2 * time.Second

func Check(listenAddr string) error {
	host, port, err := net.SplitHostPort(listenAddr)
	if err != nil {
		return fmt.Errorf("invalid listen address %q: %w", listenAddr, err)
	}

	if host == "" || host == "0.0.0.0" {
		host = "127.0.0.1"
	}

	healthURL := "http://" + net.JoinHostPort(host, port) + "/health"

	client := http.Client{Timeout: checkTimeout}
	response, err := client.Get(healthURL)
	if err != nil {
		return err
	}
	defer func() { _ = response.Body.Close() }()

	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("GET %s responded %d", healthURL, response.StatusCode)
	}

	return nil
}
