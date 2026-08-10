package main

import (
	"net/url"
	"os"
	"strings"
)

const (
	defaultCloudURL      = "https://cloud.unitx.pro"
	cloudTokenPrefix     = "utx1_"
	cloudTokenPayloadLen = 86
)

var CLOUD_URL = configuredCloudURL()

type CloudTokenResponse struct {
	ReadToken    string `json:"read_token"`
	WriteToken   string `json:"write_token"`
	DashboardURL string `json:"dashboard_url"`
}

func configuredCloudURL() string {
	if value := strings.TrimRight(strings.TrimSpace(os.Getenv("WDTMON4_CLOUD_URL")), "/"); value != "" {
		return value
	}
	return defaultCloudURL
}

func isCloudToken(token string) bool {
	token = strings.TrimSpace(token)
	if len(token) != len(cloudTokenPrefix)+cloudTokenPayloadLen {
		return false
	}
	if !strings.HasPrefix(token, cloudTokenPrefix) {
		return false
	}
	for _, ch := range token[len(cloudTokenPrefix):] {
		if (ch >= 'A' && ch <= 'Z') || (ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') || ch == '_' || ch == '-' {
			continue
		}
		return false
	}
	return true
}

func isCloudDeviceID(deviceID string) bool {
	if len(deviceID) < 1 || len(deviceID) > 64 {
		return false
	}
	for _, ch := range deviceID {
		if (ch >= 'A' && ch <= 'Z') || (ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') || ch == '_' || ch == '-' {
			continue
		}
		return false
	}
	return true
}

func cloudWriteToken(settings *Settings) string {
	if settings == nil {
		return ""
	}
	if token := strings.TrimSpace(settings.ConWriteToken); isCloudToken(token) {
		return token
	}
	if token := strings.TrimSpace(settings.ConUID); isCloudToken(token) {
		return token
	}
	return ""
}

func cloudStateURL(deviceID string) string {
	return CLOUD_URL + "/state/" + url.PathEscape(deviceID)
}
