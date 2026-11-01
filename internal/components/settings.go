package components

import (
	"errors"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/bidirekt/broker/internal/validations"
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

type Settings struct {
	PublicURL     string
	AdminUsername string
	AdminPassword string
}

func readSettings() (Settings, error) {
	publicURL, err := publicURL()
	if err != nil {
		return Settings{}, err
	}

	adminUsername, err := adminUsername()
	if err != nil {
		return Settings{}, err
	}

	adminPassword, err := adminPassword()
	if err != nil {
		return Settings{}, err
	}

	return Settings{PublicURL: publicURL, AdminUsername: adminUsername, AdminPassword: adminPassword}, nil
}

var defaultPorts = map[string]string{"https": "443", "http": "80"}

func publicURL() (string, error) {
	value := os.Getenv("BIDIREKT_PUBLIC_URL")
	if value == "" {
		return "", errors.New("BIDIREKT_PUBLIC_URL is required")
	}

	if !strings.HasPrefix(value, "https://") && !strings.HasPrefix(value, "http://") {
		return "", errors.New("BIDIREKT_PUBLIC_URL must use https, or http on localhost or 127.0.0.1")
	}

	parsed, err := url.Parse(value)
	if err != nil || parsed.Hostname() == "" || strings.HasSuffix(parsed.Host, ":") || parsed.Scheme+"://"+parsed.Host != value {
		return "", errors.New("BIDIREKT_PUBLIC_URL must be an origin (scheme://host[:port]) without a path, query, fragment or userinfo")
	}

	if parsed.Host != strings.ToLower(parsed.Host) {
		return "", errors.New("BIDIREKT_PUBLIC_URL must have a lower-case host")
	}

	if port := parsed.Port(); port != "" {
		number, err := strconv.Atoi(port)
		if err != nil || strconv.Itoa(number) != port || number < 1 || number > 65535 {
			return "", errors.New("BIDIREKT_PUBLIC_URL must have a port from 1 to 65535 without leading zeros")
		}

		if port == defaultPorts[parsed.Scheme] {
			return "", errors.New("BIDIREKT_PUBLIC_URL must not write out the default port")
		}
	}

	if parsed.Scheme == "http" && parsed.Hostname() != "localhost" && parsed.Hostname() != "127.0.0.1" {
		return "", errors.New("BIDIREKT_PUBLIC_URL must use https, or http on localhost or 127.0.0.1")
	}

	return value, nil
}

func adminUsername() (string, error) {
	value := os.Getenv("BIDIREKT_ADMIN_USERNAME")
	if value == "" {
		return "", errors.New("BIDIREKT_ADMIN_USERNAME is required")
	}

	if strings.Contains(value, "@") {
		return "", errors.New("BIDIREKT_ADMIN_USERNAME must not contain @")
	}

	return value, nil
}

func adminPassword() (string, error) {
	value := os.Getenv("BIDIREKT_ADMIN_PASSWORD")
	if value == "" {
		return "", errors.New("BIDIREKT_ADMIN_PASSWORD is required")
	}

	if err := validations.Password(value); err != nil {
		return "", errors.New("BIDIREKT_ADMIN_PASSWORD " + err.Error())
	}

	return value, nil
}
