package main

import (
	"encoding/json"
	"os"

	"github.com/datumbrain/nulltypes"
)

type Settings struct {
	Net             string
	NetEn           bool
	Proc            string
	ProcEn          bool
	Diode           bool
	Pause           bool
	ConEn           bool
	ConUID          string
	ConDev          string
	ConAlias        string
	ConAlert        bool
	ConAlertVal     int
	ConAlertSens    int
	ConAlertTimeout int
}

type Proc struct {
	Name string `json:"name"`
}

type ConnectState struct {
	Type       int                   `json:"type" form:"type"`
	Value1     nulltypes.NullFloat64 `json:"value1,omitempty" form:"value1"`
	Value2     int64                 `json:"value2,omitempty" form:"value2"`
	Alias      string                `json:"alias,omitempty" form:"alias"`
	Alert      bool                  `json:"alert,omitempty" form:"alert"`
	Alert_time int                   `json:"alert_time,omitempty" form:"alert_time"`
}

// Write сохраняет настройки в файл
func (s *Settings) Write() error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(SETTINGS_FILE, data, 0644)
}

// Read загружает настройки из файла
func (s *Settings) Read() error {
	data, err := os.ReadFile(SETTINGS_FILE)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, s)
}
