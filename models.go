package main

import (
	"database/sql"
	"encoding/json"
	"reflect"
)

type NullFloat64 sql.NullFloat64

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

// MarshalJSON for NullFloat64
func (nf *NullFloat64) MarshalJSON() ([]byte, error) {
	if !nf.Valid {
		return []byte("null"), nil
	}
	return json.Marshal(nf.Float64)
}

// UnmarshalJSON for NullFloat64
func (nf *NullFloat64) UnmarshalJSON(b []byte) error {
	err := json.Unmarshal(b, &nf.Float64)
	nf.Valid = (err == nil)
	return err
}

func (nf *NullFloat64) Scan(value interface{}) error {
	var f sql.NullFloat64
	if err := f.Scan(value); err != nil {
		return err
	}

	// if nil then make Valid false
	if reflect.TypeOf(value) == nil {
		*nf = NullFloat64{f.Float64, false}
	} else {
		*nf = NullFloat64{f.Float64, true}
	}

	return nil
}
