package config_test

import (
	"testing"

	"github.com/bidirekt/broker/internal/components/config"
	"github.com/stretchr/testify/assert"
)

func setValidAuthSettings(t *testing.T) {
	t.Setenv("BIDIREKT_PUBLIC_URL", "https://broker.example.com")
}

func TestRead_RefusesAPublicURLThatIsNotAnAllowedOrigin(t *testing.T) {
	const originRule = "BIDIREKT_PUBLIC_URL must be an origin (scheme://host[:port]) without a path, query, fragment or userinfo"
	const schemeRule = "BIDIREKT_PUBLIC_URL must use https, or http on localhost or 127.0.0.1"

	for _, refusal := range []struct {
		publicURL string
		message   string
	}{
		{"", "BIDIREKT_PUBLIC_URL is required"},
		{"https://broker.example.com/broker", originRule},
		{"https://broker.example.com/", originRule},
		{"https://broker.example.com?team=a", originRule},
		{"https://broker.example.com?", originRule},
		{"https://broker.example.com#top", originRule},
		{"https://user:pass@broker.example.com", originRule},
		{"https://broker.example.com:", originRule},
		{"https://", originRule},
		{"https://bücher.example", "BIDIREKT_PUBLIC_URL must have an ASCII host (write an international name in its xn-- form)"},
		{"https://Broker.Example.com", "BIDIREKT_PUBLIC_URL must have a lower-case host"},
		{"https://broker.example.com:443", "BIDIREKT_PUBLIC_URL must not write out the default port"},
		{"http://localhost:80", "BIDIREKT_PUBLIC_URL must not write out the default port"},
		{"https://broker.example.com:08443", "BIDIREKT_PUBLIC_URL must have a port from 1 to 65535 without leading zeros"},
		{"https://broker.example.com:0", "BIDIREKT_PUBLIC_URL must have a port from 1 to 65535 without leading zeros"},
		{"http://broker.example.com", schemeRule},
		{"http://localhost.evil.com", schemeRule},
		{"http://127.0.0.2:8080", schemeRule},
		{"http://[::1]:8080", schemeRule},
		{"HTTPS://broker.example.com", schemeRule},
		{"ftp://broker.example.com", schemeRule},
		{"broker.example.com", schemeRule},
	} {
		t.Run(refusal.publicURL, func(t *testing.T) {
			setValidAuthSettings(t)
			t.Setenv("BIDIREKT_PUBLIC_URL", refusal.publicURL)

			_, err := config.Read()

			assert.EqualError(t, err, refusal.message)
		})
	}
}

func TestRead_AcceptsAnAllowedPublicURL(t *testing.T) {
	for _, publicURL := range []string{"https://broker.example.com", "https://broker.example.com:8443", "http://localhost:8080", "http://127.0.0.1:8080", "http://localhost", "https://xn--bcher-kva.example"} {
		t.Run(publicURL, func(t *testing.T) {
			setValidAuthSettings(t)
			t.Setenv("BIDIREKT_PUBLIC_URL", publicURL)

			_, err := config.Read()

			assert.NoError(t, err)
		})
	}
}

func TestRead_RefusesTrustedProxiesWithAnEntryThatIsNotAnIPOrCIDR(t *testing.T) {
	const proxiesRule = "BIDIREKT_TRUSTED_PROXIES must be a comma-separated list of IPs or CIDRs, without spaces or empty entries"

	for _, trustedProxies := range []string{
		"proxy.example.com",
		"10.0.0.256",
		"10.0.0.0/33",
		"10.0.0.1/",
		"10.0.0.1,",
		",10.0.0.1",
		"10.0.0.1,,10.0.0.2",
		"10.0.0.1, 10.0.0.2",
		" 10.0.0.1",
		"10.0.0.1 ",
		"fe80::1%eth0",
	} {
		t.Run(trustedProxies, func(t *testing.T) {
			setValidAuthSettings(t)
			t.Setenv("BIDIREKT_TRUSTED_PROXIES", trustedProxies)

			_, err := config.Read()

			assert.EqualError(t, err, proxiesRule)
		})
	}
}

func TestRead_AcceptsTrustedProxiesThatAreIPsOrCIDRs(t *testing.T) {
	for _, trustedProxies := range []string{"", "10.0.0.1", "10.0.0.0/8,192.168.1.1", "::1", "fd00::/8,0.0.0.0"} {
		t.Run(trustedProxies, func(t *testing.T) {
			setValidAuthSettings(t)
			t.Setenv("BIDIREKT_TRUSTED_PROXIES", trustedProxies)

			_, err := config.Read()

			assert.NoError(t, err)
		})
	}
}
