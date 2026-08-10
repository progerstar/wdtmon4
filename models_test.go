package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
)

func useTempWorkingDirectory(t *testing.T) {
	t.Helper()
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(originalDir); err != nil {
			t.Errorf("restore working directory: %v", err)
		}
	})
}

func TestSettingsMarshalsPersistedWriteToken(t *testing.T) {
	settings := Settings{
		ConWriteToken: "write-token",
	}

	data, err := json.Marshal(settings)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	encoded := string(data)
	for _, forbidden := range []string{"ConReadToken", "ConDashboardURL"} {
		if strings.Contains(encoded, forbidden) {
			t.Fatalf("settings JSON contains %q: %s", forbidden, encoded)
		}
	}
	if !strings.Contains(encoded, "write-token") {
		t.Fatalf("settings JSON does not contain write token: %s", encoded)
	}
}

func TestConnectStateDistinguishesMissingAndZeroValue2(t *testing.T) {
	zero := int64(0)
	tests := []struct {
		name       string
		state      ConnectState
		wantValue2 bool
	}{
		{name: "missing", state: ConnectState{Type: 1}, wantValue2: false},
		{name: "zero", state: ConnectState{Type: 5, Value2: &zero}, wantValue2: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			data, err := json.Marshal(test.state)
			if err != nil {
				t.Fatalf("Marshal failed: %v", err)
			}

			var encoded map[string]json.RawMessage
			if err := json.Unmarshal(data, &encoded); err != nil {
				t.Fatalf("Unmarshal encoded state: %v", err)
			}
			value, hasValue2 := encoded["v2"]
			if hasValue2 != test.wantValue2 {
				t.Fatalf("v2 presence = %v, want %v: %s", hasValue2, test.wantValue2, data)
			}
			if hasValue2 && string(value) != "0" {
				t.Fatalf("v2 = %s, want 0", value)
			}
		})
	}
}

func TestSettingsWriteIsAtomicUnderConcurrentWrites(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "settings.json")

	var wg sync.WaitGroup
	errs := make(chan error, 16)
	for i := 0; i < cap(errs); i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			settings := Settings{Net: fmt.Sprintf("host-%d", i), ConDev: fmt.Sprintf("device-%d", i)}
			if err := settings.writeTo(filename); err != nil {
				errs <- err
			}
		}(i)
	}
	wg.Wait()
	close(errs)

	for err := range errs {
		t.Fatal(err)
	}

	data, err := os.ReadFile(filename)
	if err != nil {
		t.Fatal(err)
	}
	var saved Settings
	if err := json.Unmarshal(data, &saved); err != nil {
		t.Fatalf("settings file is not valid JSON: %v", err)
	}
	if saved.Net == "" || saved.ConDev == "" {
		t.Fatalf("unexpected settings after concurrent writes: %+v", saved)
	}
	if runtime.GOOS != "windows" {
		info, err := os.Stat(filename)
		if err != nil {
			t.Fatal(err)
		}
		if got := info.Mode().Perm(); got != settingsFileMode {
			t.Fatalf("settings mode = %04o, want %04o", got, settingsFileMode)
		}
	}

	tempFiles, err := filepath.Glob(filepath.Join(filepath.Dir(filename), ".settings.json.tmp-*"))
	if err != nil {
		t.Fatal(err)
	}
	if len(tempFiles) != 0 {
		t.Fatalf("temporary settings files were not cleaned up: %v", tempFiles)
	}
}

func TestSettingsReadTightensExistingFilePermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows does not expose Unix permission bits")
	}
	useTempWorkingDirectory(t)
	if err := os.WriteFile(SETTINGS_FILE, []byte(`{"ConDev":"device-1"}`), 0644); err != nil {
		t.Fatal(err)
	}

	var settings Settings
	if err := settings.Read(); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(SETTINGS_FILE)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != settingsFileMode {
		t.Fatalf("settings mode = %04o, want %04o", got, settingsFileMode)
	}
}

func TestInitSettingsDoesNotOverwriteInvalidExistingFile(t *testing.T) {
	useTempWorkingDirectory(t)

	invalid := []byte("{not-json")
	if err := os.WriteFile(SETTINGS_FILE, invalid, 0644); err != nil {
		t.Fatal(err)
	}

	app := NewApp()
	defer app.cancel()
	if err := app.initSettings(); err == nil {
		t.Fatal("initSettings succeeded for invalid settings.json")
	}
	actual, err := os.ReadFile(SETTINGS_FILE)
	if err != nil {
		t.Fatal(err)
	}
	if string(actual) != string(invalid) {
		t.Fatalf("invalid settings file was overwritten: %q", actual)
	}
}
