package main

import (
	"encoding/json"
	"os"
	"strconv"
	"strings"
	"testing"
)

func TestParseArgsAllowsVersionWithoutPort(t *testing.T) {
	originalArgs := os.Args
	os.Args = []string{"wdtmon4", "--version"}
	defer func() { os.Args = originalArgs }()

	_, _, _, showVersion, _, _, err := parseArgs()
	if err != nil {
		t.Fatalf("parseArgs returned an error for --version: %v", err)
	}
	if !showVersion {
		t.Fatal("parseArgs did not recognize --version")
	}
}

func TestParseArgsDefaultsToAutoPortAndBrowser(t *testing.T) {
	originalArgs := os.Args
	os.Args = []string{"wdtmon4"}
	defer func() { os.Args = originalArgs }()

	portName, headless, _, showVersion, host, hport, err := parseArgs()
	if err != nil {
		t.Fatalf("parseArgs returned an error without a port: %v", err)
	}
	if portName != "" {
		t.Fatalf("portName = %q, want auto-detection", portName)
	}
	if headless {
		t.Fatal("browser opening is disabled by default")
	}
	if showVersion {
		t.Fatal("showVersion is enabled by default")
	}
	if host != "127.0.0.1" || hport != "8000" {
		t.Fatalf("HTTP address = %s:%s, want 127.0.0.1:8000", host, hport)
	}
}

func TestParseArgsAcceptsExplicitPortAndHeadlessMode(t *testing.T) {
	originalArgs := os.Args
	os.Args = []string{"wdtmon4", "--headless", "--host", "0.0.0.0", "explicit-port"}
	defer func() { os.Args = originalArgs }()

	portName, headless, _, _, host, _, err := parseArgs()
	if err != nil {
		t.Fatal(err)
	}
	if portName != "explicit-port" {
		t.Fatalf("portName = %q, want explicit-port", portName)
	}
	if !headless {
		t.Fatal("--headless was not recognized")
	}
	if host != "0.0.0.0" {
		t.Fatalf("host = %q, want 0.0.0.0", host)
	}
}

func TestParseArgsRejectsUnsafeEmptyHost(t *testing.T) {
	for _, host := range []string{"", " localhost", "localhost\n"} {
		t.Run(strconv.Quote(host), func(t *testing.T) {
			originalArgs := os.Args
			os.Args = []string{"wdtmon4", "--host", host}
			defer func() { os.Args = originalArgs }()

			if _, _, _, _, _, _, err := parseArgs(); err == nil {
				t.Fatalf("parseArgs accepted host %q", host)
			}
		})
	}
}

func TestParseArgsRetainsWebFlagCompatibility(t *testing.T) {
	originalArgs := os.Args
	os.Args = []string{"wdtmon4", "-w"}
	defer func() { os.Args = originalArgs }()

	portName, headless, _, _, _, _, err := parseArgs()
	if err != nil {
		t.Fatal(err)
	}
	if portName != "" || headless {
		t.Fatalf("-w changed defaults: portName=%q headless=%v", portName, headless)
	}
}

func TestInitSettingsMigratesEmptyCloudDeviceID(t *testing.T) {
	useTempWorkingDirectory(t)
	if err := os.WriteFile(SETTINGS_FILE, []byte(`{"ConDev":"","Diode":true}`), 0644); err != nil {
		t.Fatal(err)
	}

	app := NewApp()
	defer app.cancel()
	if err := app.initSettings(); err != nil {
		t.Fatal(err)
	}
	if !isCloudDeviceID(app.settings.ConDev) {
		t.Fatalf("migrated device ID = %q", app.settings.ConDev)
	}

	data, err := os.ReadFile(SETTINGS_FILE)
	if err != nil {
		t.Fatal(err)
	}
	var persisted Settings
	if err := json.Unmarshal(data, &persisted); err != nil {
		t.Fatal(err)
	}
	if persisted.ConDev != app.settings.ConDev {
		t.Fatalf("persisted device ID = %q, want %q", persisted.ConDev, app.settings.ConDev)
	}
}

func TestCloudStartupOverrideEnablesSession(t *testing.T) {
	app := NewApp()
	defer app.cancel()
	app.settings.ConWriteToken = cloudTokenPrefix + strings.Repeat("a", cloudTokenPayloadLen)
	app.settings.ConEn = false

	app.applyCloudStartupOverride(true)
	if !cloudEnabled(app.settings) {
		t.Fatal("--cloud startup override did not enable cloud delivery")
	}
}
