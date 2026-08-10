package main

import (
	"fmt"
	"net"
	"testing"
)

func TestNetworkProbeAddress(t *testing.T) {
	tests := []struct {
		name   string
		target string
		want   string
	}{
		{name: "plain host keeps port 80 compatibility", target: "example.com", want: "example.com:80"},
		{name: "host and port", target: "example.com:8080", want: "example.com:8080"},
		{name: "HTTP default port", target: "http://example.com/health", want: "example.com:80"},
		{name: "HTTPS default port", target: "https://example.com/health", want: "example.com:443"},
		{name: "HTTPS explicit port", target: "https://example.com:8443/health", want: "example.com:8443"},
		{name: "TCP URL", target: "tcp://example.com:9000", want: "example.com:9000"},
		{name: "IPv6 literal", target: "2001:db8::1", want: "[2001:db8::1]:80"},
		{name: "IPv6 URL", target: "https://[2001:db8::1]/", want: "[2001:db8::1]:443"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := networkProbeAddress(tt.target)
			if err != nil {
				t.Fatalf("networkProbeAddress(%q) returned an error: %v", tt.target, err)
			}
			if got != tt.want {
				t.Fatalf("networkProbeAddress(%q) = %q, want %q", tt.target, got, tt.want)
			}
		})
	}
}

func TestNetworkProbeAddressRejectsInvalidTargets(t *testing.T) {
	for _, target := range []string{
		"",
		"host:invalid",
		"https://example.com:invalid",
		"tcp://example.com",
		"bad host",
		"example.com/path",
	} {
		t.Run(target, func(t *testing.T) {
			if _, err := networkProbeAddress(target); err == nil {
				t.Fatalf("networkProbeAddress(%q) succeeded, want an error", target)
			}
		})
	}
}

func TestNetworkProbeAddressRejectsASCIIControlCharacters(t *testing.T) {
	for control := 0; control <= 0x1f; control++ {
		t.Run(fmt.Sprintf("0x%02x", control), func(t *testing.T) {
			target := "example" + string(rune(control)) + ".com"
			if _, err := networkProbeAddress(target); err == nil {
				t.Fatalf("networkProbeAddress(%q) succeeded, want an error", target)
			}
		})
	}

	t.Run("DEL", func(t *testing.T) {
		target := "example" + string(rune(0x7f)) + ".com"
		if _, err := networkProbeAddress(target); err == nil {
			t.Fatalf("networkProbeAddress(%q) succeeded, want an error", target)
		}
	})
}

func TestPingUsesConfiguredTCPPort(t *testing.T) {
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()

	if err := ping("tcp://" + listener.Addr().String()); err != nil {
		t.Fatalf("ping did not connect to configured TCP endpoint: %v", err)
	}
}
