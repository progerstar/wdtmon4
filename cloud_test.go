package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/datumbrain/nulltypes"
)

func validCloudToken() string {
	return cloudTokenPrefix + strings.Repeat("a", cloudTokenPayloadLen)
}

func TestConfiguredCloudURLUsesOnlyNamespacedOverride(t *testing.T) {
	t.Setenv("WDTMON4_CLOUD_URL", "")
	t.Setenv("CLOUD_URL", "https://attacker.example")
	if got := configuredCloudURL(); got != defaultCloudURL {
		t.Fatalf("generic CLOUD_URL changed endpoint to %q", got)
	}

	t.Setenv("WDTMON4_CLOUD_URL", " https://staging.unitx.pro/ ")
	if got := configuredCloudURL(); got != "https://staging.unitx.pro" {
		t.Fatalf("namespaced override = %q", got)
	}
}

func requireCloudValue2(t *testing.T, state ConnectState) int64 {
	t.Helper()
	if state.Value2 == nil {
		t.Fatal("cloud state does not contain v2")
	}
	return *state.Value2
}

func TestCloudEnabledRequiresSettingAndAValidToken(t *testing.T) {
	settings := &Settings{ConEn: true, ConWriteToken: validCloudToken(), ConDev: "device-1"}

	if !cloudEnabled(settings) {
		t.Fatal("enabled cloud settings were rejected")
	}
	settings.ConEn = false
	if cloudEnabled(settings) {
		t.Fatal("cloud is enabled despite ConEn being false")
	}
	settings.ConEn = true
	settings.ConWriteToken = "invalid"
	if cloudEnabled(settings) {
		t.Fatal("cloud is enabled with an invalid token")
	}
	settings.ConWriteToken = validCloudToken()
	settings.ConDev = ""
	if cloudEnabled(settings) {
		t.Fatal("cloud is enabled without a device ID")
	}
	settings.ConDev = "invalid/device"
	if cloudEnabled(settings) {
		t.Fatal("cloud is enabled with an invalid device ID")
	}
}

func TestCurrentCloudStateReportsInactiveDevice(t *testing.T) {
	state := currentCloudState(false, invalidTemperature())
	if state.Type != 5 || requireCloudValue2(t, state) != 0 || state.Value1.Valid {
		t.Fatalf("unexpected inactive cloud state: %+v", state)
	}

	state = currentCloudState(true, nulltypes.NullFloat64{Float64: 23.5, Valid: true})
	if requireCloudValue2(t, state) != 1 || !state.Value1.Valid || state.Value1.Float64 != 23.5 {
		t.Fatalf("unexpected active cloud state: %+v", state)
	}
}

func TestConsendRequiresCloudNoContentResponse(t *testing.T) {
	for _, status := range []int{http.StatusOK, http.StatusAccepted, http.StatusMultipleChoices} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(status)
			}))
			defer server.Close()

			state := currentCloudState(true, invalidTemperature())
			err := consend(context.Background(), validCloudToken(), server.URL, &state, server.Client())
			var responseError *cloudHTTPError
			if !errors.As(err, &responseError) || responseError.StatusCode != status {
				t.Fatalf("consend accepted status %d or returned the wrong error: %v", status, err)
			}
		})
	}
}

func TestConsendUsesWriteTokenAndReportsCloudErrorCode(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", req.Method)
		}
		if got := req.Header.Get("Authorization"); got != "Bearer "+validCloudToken() {
			t.Errorf("Authorization = %q", got)
		}

		var state ConnectState
		if err := json.NewDecoder(req.Body).Decode(&state); err != nil {
			t.Errorf("decode state: %v", err)
		}
		if state.Type != 5 || requireCloudValue2(t, state) != 0 {
			t.Errorf("unexpected state: %+v", state)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"error":"forbidden_scope"}`))
	}))
	defer server.Close()

	state := currentCloudState(false, invalidTemperature())
	err := consend(context.Background(), validCloudToken(), server.URL, &state, server.Client())
	var responseError *cloudHTTPError
	if !errors.As(err, &responseError) {
		t.Fatalf("consend error type = %T, want *cloudHTTPError", err)
	}
	if responseError.StatusCode != http.StatusForbidden || responseError.Code != "forbidden_scope" {
		t.Fatalf("unexpected cloud error: %+v", responseError)
	}
}

func TestConsendAcceptsNoContent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	state := currentCloudState(true, invalidTemperature())
	if err := consend(context.Background(), validCloudToken(), server.URL, &state, server.Client()); err != nil {
		t.Fatalf("consend rejected 204 response: %v", err)
	}
}

func TestPerioderUploadsInactiveState(t *testing.T) {
	originalCloudURL := CLOUD_URL
	defer func() { CLOUD_URL = originalCloudURL }()

	states := make(chan ConnectState, 4)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		var state ConnectState
		if err := json.NewDecoder(req.Body).Decode(&state); err != nil {
			t.Errorf("decode state: %v", err)
		} else {
			states <- state
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	CLOUD_URL = server.URL

	settings := &Settings{
		ConEn:         true,
		ConWriteToken: validCloudToken(),
		ConDev:        "inactive-device",
	}
	var mu sync.RWMutex
	active := false
	temperature := invalidTemperature()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		perioderWithIntervals(
			ctx,
			settings,
			&mu,
			make(chan serialRequest),
			&active,
			&temperature,
			time.Hour,
			50*time.Millisecond,
		)
		close(done)
	}()

	select {
	case initial := <-states:
		if value := requireCloudValue2(t, initial); value != 2 {
			t.Fatalf("initial connection state = %d, want 2", value)
		}
	case <-time.After(time.Second):
		t.Fatal("perioder did not send initial cloud state")
	}

	select {
	case inactive := <-states:
		if requireCloudValue2(t, inactive) != 0 || inactive.Value1.Valid {
			t.Fatalf("inactive cloud state = %+v", inactive)
		}
	case <-time.After(time.Second):
		t.Fatal("perioder skipped inactive cloud state")
	}

	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("perioder did not stop after context cancellation")
	}
}

func TestBlockedCloudDoesNotDelayHeartbeat(t *testing.T) {
	originalCloudURL := CLOUD_URL
	defer func() { CLOUD_URL = originalCloudURL }()

	requestStarted := make(chan struct{})
	releaseRequest := make(chan struct{})
	var requestOnce sync.Once
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, req *http.Request) {
		requestOnce.Do(func() { close(requestStarted) })
		select {
		case <-req.Context().Done():
		case <-releaseRequest:
		}
	}))
	defer server.Close()
	defer close(releaseRequest)
	CLOUD_URL = server.URL

	settings := &Settings{
		ConEn:         true,
		ConWriteToken: validCloudToken(),
		ConDev:        "blocked-cloud-device",
	}
	var mu sync.RWMutex
	active := false
	temperature := invalidTemperature()
	serialRequests := make(chan serialRequest)
	heartbeatSeen := make(chan time.Time, 1)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case request := <-serialRequests:
				switch request.command {
				case "~U":
					select {
					case heartbeatSeen <- time.Now():
					default:
					}
					request.respond("~A")
				case "~G":
					request.respond("~G0235")
				default:
					request.respond("")
				}
			}
		}
	}()

	go func() {
		perioderWithIntervals(
			ctx,
			settings,
			&mu,
			serialRequests,
			&active,
			&temperature,
			20*time.Millisecond,
			time.Hour,
		)
		close(done)
	}()

	select {
	case <-requestStarted:
	case <-time.After(time.Second):
		cancel()
		t.Fatal("perioder did not start cloud request")
	}
	blockedAt := time.Now()

	select {
	case heartbeatAt := <-heartbeatSeen:
		if delay := heartbeatAt.Sub(blockedAt); delay > 250*time.Millisecond {
			t.Fatalf("heartbeat delayed by blocked cloud request: %v", delay)
		}
	case <-time.After(250 * time.Millisecond):
		cancel()
		t.Fatal("blocked cloud request delayed serial heartbeat")
	}

	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("perioder did not stop blocked cloud request after cancellation")
	}
}
