package main

import (
	"encoding/json"
	"io/ioutil"

	"github.com/datumbrain/nulltypes"
)

type NullFloat64 = nulltypes.NullFloat64

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
	Name string
}

type ConnectState struct {
	Type       int         `json:"type" form:"type"`
	Value1     NullFloat64 `json:"value1,omitempty" form:"value1"`
	Value2     int64       `json:"value2,omitempty" form:"value2"`
	Alias      string      `json:"alias,omitempty" form:"alias"`
	Alert      bool        `json:"alert,omitempty" form:"alert"`
	Alert_time int         `json:"alert_time,omitempty" form:"alert_time"`
}

// for Settings
func (s Settings) Write() error {
	data, err := json.Marshal(s)
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(SETTINGS_FILE, data, 0644)
	if err != nil {
		return err
	}
	return nil
}

func (s *Settings) Read() error {
	data, err := ioutil.ReadFile(SETTINGS_FILE)
	if err == nil {
		err = json.Unmarshal(data, &s)
	}
	return err
}
