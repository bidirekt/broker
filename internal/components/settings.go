package components

import (
	"errors"
	"os"
	"strconv"
)

const defaultListenAddr = ":8080"

func ListenAddr() string {
	if listenAddr := os.Getenv("BIDIREKT_LISTEN_ADDR"); listenAddr != "" {
		return listenAddr
	}

	return defaultListenAddr
}

func databaseURL() (string, error) {
	value := os.Getenv("BIDIREKT_DATABASE_URL")
	if value == "" {
		return "", errors.New("BIDIREKT_DATABASE_URL is required")
	}

	return value, nil
}

const defaultDatabaseConnectAttempts = 10

func databaseConnectAttempts() (int, error) {
	value := os.Getenv("BIDIREKT_DATABASE_CONNECT_RETRIES")
	if value == "" {
		return defaultDatabaseConnectAttempts, nil
	}

	attempts, err := strconv.Atoi(value)
	if err != nil || attempts < 1 {
		return 0, errors.New("BIDIREKT_DATABASE_CONNECT_RETRIES must be a positive integer")
	}

	return attempts, nil
}
