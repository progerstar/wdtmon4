package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/datumbrain/nulltypes"
)

type Settings struct {
	Net           string
	NetEn         bool
	Proc          string
	ProcEn        bool
	Diode         bool
	Pause         bool
	ConEn         bool
	ConUID        string
	ConWriteToken string
	ConDev        string
}

type settingsResponse struct {
	Net           string
	NetEn         bool
	Proc          string
	ProcEn        bool
	Diode         bool
	Pause         bool
	ConEn         bool
	ConConfigured bool
	ConDev        string
}

type Proc struct {
	Name string `json:"name"`
}

type ConnectState struct {
	Type   int                   `json:"ty" form:"ty"`
	Value1 nulltypes.NullFloat64 `json:"v1,omitempty" form:"v1"`
	Value2 *int64                `json:"v2,omitempty" form:"v2"`
}

const settingsFileMode os.FileMode = 0600

// Write persists settings atomically with owner-only permissions.
func (s *Settings) Write() error {
	return s.writeTo(SETTINGS_FILE)
}

func (s *Settings) writeTo(filename string) error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomically(filename, data, settingsFileMode)
}

func writeFileAtomically(filename string, data []byte, mode os.FileMode) (err error) {
	dir := filepath.Dir(filename)
	temp, err := os.CreateTemp(dir, "."+filepath.Base(filename)+".tmp-*")
	if err != nil {
		return fmt.Errorf("create temporary settings file: %w", err)
	}
	tempName := temp.Name()
	defer func() {
		if temp != nil {
			_ = temp.Close()
		}
		_ = os.Remove(tempName)
	}()

	if err := temp.Chmod(mode); err != nil {
		return fmt.Errorf("set temporary settings permissions: %w", err)
	}
	if _, err := temp.Write(data); err != nil {
		return fmt.Errorf("write temporary settings file: %w", err)
	}
	if err := temp.Sync(); err != nil {
		return fmt.Errorf("sync temporary settings file: %w", err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("close temporary settings file: %w", err)
	}
	temp = nil

	if err := replaceFile(tempName, filename); err != nil {
		return fmt.Errorf("replace settings file: %w", err)
	}
	return nil
}

// Read loads settings and tightens permissions on a valid existing file.
func (s *Settings) Read() error {
	data, err := os.ReadFile(SETTINGS_FILE)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, s); err != nil {
		return err
	}
	if err := os.Chmod(SETTINGS_FILE, settingsFileMode); err != nil {
		return fmt.Errorf("secure settings file permissions: %w", err)
	}
	return nil
}
