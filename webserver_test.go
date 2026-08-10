package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/datumbrain/nulltypes"
	"github.com/gofiber/fiber/v2"
)

func newTestWebServer() (*WebServer, *sync.RWMutex) {
	return newTestWebServerForHost("127.0.0.1")
}

func newTestWebServerForHost(host string) (*WebServer, *sync.RWMutex) {
	settings := &Settings{}
	var mu sync.RWMutex
	active := true
	temperature := nulltypes.NullFloat64{Float64: 23.5, Valid: true}
	return NewWebServer(settings, &mu, make(chan serialRequest), &active, &temperature, host), &mu
}

func requireSecurityHeaders(t *testing.T, response *http.Response) {
	t.Helper()
	want := map[string]string{
		"Content-Security-Policy": "frame-ancestors 'none'",
		"X-Frame-Options":         "DENY",
		"X-Content-Type-Options":  "nosniff",
		"Referrer-Policy":         "no-referrer",
	}
	for name, value := range want {
		if got := response.Header.Get(name); got != value {
			t.Errorf("%s = %q, want %q", name, got, value)
		}
	}
}

func TestValidateSettingsChecksNetworkTarget(t *testing.T) {
	ws, _ := newTestWebServer()
	validDeviceID := "device-1"

	if err := ws.validateSettings(&Settings{NetEn: true, Net: "https://example.com", ConDev: validDeviceID}); err != nil {
		t.Fatalf("valid HTTPS target was rejected: %v", err)
	}
	if err := ws.validateSettings(&Settings{NetEn: true, Net: "example.com:invalid", ConDev: validDeviceID}); err == nil {
		t.Fatal("invalid TCP target was accepted")
	}
	if err := ws.validateSettings(&Settings{NetEn: true, ConDev: validDeviceID}); err == nil {
		t.Fatal("empty enabled network target was accepted")
	}
	if err := ws.validateSettings(&Settings{}); err == nil {
		t.Fatal("empty device ID was accepted")
	}
}

func TestStatusAndCachedCommandRoutes(t *testing.T) {
	ws, _ := newTestWebServer()
	app := fiber.New()
	ws.setupAPIRoutes(app)

	statusResponse, err := app.Test(httptest.NewRequest("GET", "/status", nil))
	if err != nil {
		t.Fatal(err)
	}
	defer statusResponse.Body.Close()
	if statusResponse.StatusCode != fiber.StatusOK {
		t.Fatalf("GET /status returned %d", statusResponse.StatusCode)
	}

	var status struct {
		Active      bool                  `json:"active"`
		Temperature nulltypes.NullFloat64 `json:"temperature"`
		Uptime      int64                 `json:"uptime"`
		Version     string                `json:"version"`
	}
	if err := json.NewDecoder(statusResponse.Body).Decode(&status); err != nil {
		t.Fatal(err)
	}
	if !status.Active || !status.Temperature.Valid || status.Temperature.Float64 != 23.5 || status.Version != VERSION {
		t.Fatalf("unexpected status response: %+v", status)
	}

	commandResponse, err := app.Test(httptest.NewRequest(http.MethodPost, "/cmd/~U", nil))
	if err != nil {
		t.Fatal(err)
	}
	defer commandResponse.Body.Close()
	body, err := io.ReadAll(commandResponse.Body)
	if err != nil {
		t.Fatal(err)
	}
	if commandResponse.StatusCode != fiber.StatusOK || string(body) != "~A" {
		t.Fatalf("POST /cmd/~U returned %d %q", commandResponse.StatusCode, body)
	}

	getResponse, err := app.Test(httptest.NewRequest(http.MethodGet, "/cmd/~U", nil))
	if err != nil {
		t.Fatal(err)
	}
	defer getResponse.Body.Close()
	if getResponse.StatusCode != fiber.StatusMethodNotAllowed {
		t.Fatalf("GET /cmd/~U returned %d, want 405", getResponse.StatusCode)
	}
}

func TestRequestSecurityEnforcesOriginAndLoopbackHost(t *testing.T) {
	ws, _ := newTestWebServer()
	app := fiber.New()
	ws.setupRoutes(app)

	tests := []struct {
		name       string
		host       string
		origin     string
		wantStatus int
	}{
		{name: "local host without origin", host: "127.0.0.1:8000", wantStatus: fiber.StatusOK},
		{name: "same origin", host: "localhost:8000", origin: "http://localhost:8000", wantStatus: fiber.StatusOK},
		{name: "default port normalized", host: "localhost", origin: "http://localhost:80", wantStatus: fiber.StatusOK},
		{name: "cross origin", host: "localhost:8000", origin: "https://evil.example", wantStatus: fiber.StatusForbidden},
		{name: "local aliases do not count as same origin", host: "127.0.0.1:8000", origin: "http://localhost:8000", wantStatus: fiber.StatusForbidden},
		{name: "DNS rebinding host", host: "watchdog.attacker.example:8000", origin: "http://watchdog.attacker.example:8000", wantStatus: fiber.StatusForbidden},
		{name: "malformed origin", host: "localhost:8000", origin: "null", wantStatus: fiber.StatusForbidden},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "http://"+test.host+"/status", nil)
			req.Host = test.host
			if test.origin != "" {
				req.Header.Set("Origin", test.origin)
			}
			response, err := app.Test(req)
			if err != nil {
				t.Fatal(err)
			}
			response.Body.Close()
			if response.StatusCode != test.wantStatus {
				t.Fatalf("status = %d, want %d", response.StatusCode, test.wantStatus)
			}
			if response.Header.Get("Access-Control-Allow-Origin") != "" {
				t.Fatal("response unexpectedly enables CORS")
			}
			requireSecurityHeaders(t, response)
		})
	}
}

func TestRequestSecurityAllowsExplicitRemoteBind(t *testing.T) {
	ws, _ := newTestWebServerForHost("0.0.0.0")
	app := fiber.New()
	ws.setupRoutes(app)

	req := httptest.NewRequest(http.MethodGet, "http://appliance.example/status", nil)
	req.Host = "appliance.example"
	req.Header.Set("Origin", "http://appliance.example")
	response, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != fiber.StatusOK {
		t.Fatalf("status = %d, want 200", response.StatusCode)
	}
}

func TestGetSettingsRedactsCloudCredentials(t *testing.T) {
	ws, _ := newTestWebServer()
	ws.settings.ConUID = validCloudToken()
	ws.settings.ConWriteToken = validCloudToken()
	ws.settings.ConDev = "device-1"
	app := fiber.New()
	ws.setupAPIRoutes(app)

	response, err := app.Test(httptest.NewRequest(http.MethodGet, "/settings", nil))
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"ConUID", "ConWriteToken", "ConReadToken", "ConDashboardURL", validCloudToken()} {
		if bytes.Contains(body, []byte(forbidden)) {
			t.Fatalf("settings response contains %q: %s", forbidden, body)
		}
	}
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatal(err)
	}
	if payload["ConConfigured"] != true || payload["ConDev"] != "device-1" {
		t.Fatalf("unexpected settings response: %s", body)
	}
	if got := response.Header.Get("Cache-Control"); got != "no-store" {
		t.Fatalf("Cache-Control = %q, want no-store", got)
	}
}

func TestUpdateSettingsNeverAcceptsCredentialsFromGenericEndpoint(t *testing.T) {
	existingToken := validCloudToken()
	incomingToken := cloudTokenPrefix + strings.Repeat("b", cloudTokenPayloadLen)
	tests := []struct {
		name          string
		existingWrite string
		existingUID   string
		incomingWrite string
		incomingUID   string
		wantToken     string
	}{
		{name: "canonical token is preserved", existingWrite: existingToken, incomingWrite: incomingToken, incomingUID: incomingToken, wantToken: existingToken},
		{name: "legacy token is migrated", existingUID: existingToken, wantToken: existingToken},
		{name: "generic endpoint cannot install token", incomingWrite: incomingToken, incomingUID: incomingToken, wantToken: ""},
		{name: "invalid incoming token is ignored", existingWrite: existingToken, incomingWrite: "not-a-token", wantToken: existingToken},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			useTempWorkingDirectory(t)
			ws, _ := newTestWebServer()
			ws.settings.ConWriteToken = test.existingWrite
			ws.settings.ConUID = test.existingUID
			app := fiber.New()
			ws.setupAPIRoutes(app)

			requestBody, err := json.Marshal(Settings{
				Diode:         true,
				ConDev:        "device-1",
				ConUID:        test.incomingUID,
				ConWriteToken: test.incomingWrite,
			})
			if err != nil {
				t.Fatal(err)
			}
			req := httptest.NewRequest(http.MethodPost, "/settings", bytes.NewReader(requestBody))
			req.Header.Set("Content-Type", "application/json")
			response, err := app.Test(req)
			if err != nil {
				t.Fatal(err)
			}
			response.Body.Close()
			if response.StatusCode != fiber.StatusOK {
				t.Fatalf("status = %d, want 200", response.StatusCode)
			}
			if ws.settings.ConWriteToken != test.wantToken || ws.settings.ConUID != "" {
				t.Fatalf("stored credentials = write:%q legacy:%q", ws.settings.ConWriteToken, ws.settings.ConUID)
			}

			data, err := os.ReadFile(SETTINGS_FILE)
			if err != nil {
				t.Fatal(err)
			}
			var persisted Settings
			if err := json.Unmarshal(data, &persisted); err != nil {
				t.Fatal(err)
			}
			if persisted.ConWriteToken != test.wantToken || persisted.ConUID != "" {
				t.Fatalf("persisted credentials = write:%q legacy:%q", persisted.ConWriteToken, persisted.ConUID)
			}
		})
	}
}

func TestClearCloudCredentialsIsExplicitAndPersistent(t *testing.T) {
	useTempWorkingDirectory(t)
	ws, _ := newTestWebServer()
	ws.settings.ConEn = true
	ws.settings.ConUID = validCloudToken()
	ws.settings.ConWriteToken = validCloudToken()
	ws.settings.ConDev = "device-1"
	app := fiber.New()
	ws.setupAPIRoutes(app)

	response, err := app.Test(httptest.NewRequest(http.MethodPost, "/con/clear", nil))
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != fiber.StatusNoContent {
		t.Fatalf("status = %d, want 204", response.StatusCode)
	}
	if ws.settings.ConEn || ws.settings.ConUID != "" || ws.settings.ConWriteToken != "" {
		t.Fatalf("credentials were not cleared: %+v", ws.settings)
	}

	data, err := os.ReadFile(SETTINGS_FILE)
	if err != nil {
		t.Fatal(err)
	}
	var persisted Settings
	if err := json.Unmarshal(data, &persisted); err != nil {
		t.Fatal(err)
	}
	if persisted.ConEn || persisted.ConUID != "" || persisted.ConWriteToken != "" {
		t.Fatalf("credentials were not cleared on disk: %+v", persisted)
	}
}

func TestValidateCloudTokenChecksWriteScopeWithCloud(t *testing.T) {
	useTempWorkingDirectory(t)
	originalCloudURL := CLOUD_URL
	defer func() { CLOUD_URL = originalCloudURL }()

	cloud := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path != "/state/device-1" {
			t.Errorf("cloud path = %q", req.URL.Path)
		}
		if got := req.Header.Get("Authorization"); got != "Bearer "+validCloudToken() {
			t.Errorf("Authorization = %q", got)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer cloud.Close()
	CLOUD_URL = cloud.URL

	ws, _ := newTestWebServer()
	ws.settings.ConDev = "device-1"
	ws.httpClient = cloud.Client()
	app := fiber.New()
	ws.setupAPIRoutes(app)

	body := strings.NewReader(`{"write_token":"` + validCloudToken() + `"}`)
	req := httptest.NewRequest(http.MethodPost, "/con/validate", body)
	req.Header.Set("Content-Type", "application/json")
	response, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != fiber.StatusOK {
		payload, _ := io.ReadAll(response.Body)
		t.Fatalf("POST /con/validate returned %d: %s", response.StatusCode, payload)
	}

	var result struct {
		DeviceID string `json:"device_id"`
	}
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if result.DeviceID != "device-1" {
		t.Fatalf("device_id = %q", result.DeviceID)
	}
	if ws.settings.ConWriteToken != validCloudToken() || ws.settings.ConUID != "" || !ws.settings.ConEn {
		t.Fatalf("validated credentials were not stored canonically: %+v", ws.settings)
	}
	var persisted Settings
	data, err := os.ReadFile(SETTINGS_FILE)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &persisted); err != nil {
		t.Fatal(err)
	}
	if persisted.ConWriteToken != validCloudToken() || persisted.ConDev != "device-1" || !persisted.ConEn {
		t.Fatalf("validated credentials were not persisted: %+v", persisted)
	}
}

func TestValidateCloudTokenRejectsReadScope(t *testing.T) {
	originalCloudURL := CLOUD_URL
	defer func() { CLOUD_URL = originalCloudURL }()

	cloud := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"error":"forbidden_scope"}`))
	}))
	defer cloud.Close()
	CLOUD_URL = cloud.URL

	ws, _ := newTestWebServer()
	ws.settings.ConDev = "device-1"
	ws.httpClient = cloud.Client()
	app := fiber.New()
	ws.setupAPIRoutes(app)

	body := strings.NewReader(`{"write_token":"` + validCloudToken() + `"}`)
	req := httptest.NewRequest(http.MethodPost, "/con/validate", body)
	req.Header.Set("Content-Type", "application/json")
	response, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != fiber.StatusBadRequest {
		payload, _ := io.ReadAll(response.Body)
		t.Fatalf("POST /con/validate returned %d: %s", response.StatusCode, payload)
	}
}

func TestValidateDashboardURL(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{name: "empty optional URL"},
		{name: "HTTP", value: "http://cloud.example/dashboard"},
		{name: "HTTPS with fragment", value: "https://cloud.example/#read_token=value"},
		{name: "relative", value: "/dashboard", wantErr: true},
		{name: "scheme relative", value: "//cloud.example/dashboard", wantErr: true},
		{name: "wrong scheme", value: "javascript:alert(1)", wantErr: true},
		{name: "userinfo", value: "https://user:password@cloud.example/", wantErr: true},
		{name: "missing host", value: "https:///dashboard", wantErr: true},
		{name: "surrounding whitespace", value: " https://cloud.example/", wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validateDashboardURL(test.value)
			if (err != nil) != test.wantErr {
				t.Fatalf("validateDashboardURL(%q) error = %v, wantErr %v", test.value, err, test.wantErr)
			}
		})
	}
}

func TestCreateCloudUserReturnsDeviceIDAndStoresOnlyWriteToken(t *testing.T) {
	useTempWorkingDirectory(t)
	originalCloudURL := CLOUD_URL
	defer func() { CLOUD_URL = originalCloudURL }()

	readToken := cloudTokenPrefix + strings.Repeat("r", cloudTokenPayloadLen)
	writeToken := cloudTokenPrefix + strings.Repeat("w", cloudTokenPayloadLen)
	cloud := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path != "/token" {
			t.Errorf("cloud path = %q", req.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(CloudTokenResponse{
			ReadToken:    readToken,
			WriteToken:   writeToken,
			DashboardURL: "https://cloud.unitx.pro/#read_token=value",
		})
	}))
	defer cloud.Close()
	CLOUD_URL = cloud.URL

	ws, _ := newTestWebServer()
	ws.httpClient = cloud.Client()
	app := fiber.New()
	ws.setupAPIRoutes(app)

	response, err := app.Test(httptest.NewRequest(http.MethodPost, "/con/create", nil))
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != fiber.StatusOK {
		body, _ := io.ReadAll(response.Body)
		t.Fatalf("status = %d: %s", response.StatusCode, body)
	}
	if got := response.Header.Get("Cache-Control"); got != "no-store" {
		t.Fatalf("Cache-Control = %q, want no-store", got)
	}
	var created cloudTokenCreateResponse
	if err := json.NewDecoder(response.Body).Decode(&created); err != nil {
		t.Fatal(err)
	}
	if created.ReadToken != readToken || created.WriteToken != writeToken || !isCloudDeviceID(created.DeviceID) {
		t.Fatalf("unexpected token response: %+v", created)
	}
	if ws.settings.ConWriteToken != writeToken || ws.settings.ConUID != "" || ws.settings.ConDev != created.DeviceID || !ws.settings.ConEn {
		t.Fatalf("credentials were not stored canonically: %+v", ws.settings)
	}

	data, err := os.ReadFile(SETTINGS_FILE)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(data, []byte(readToken)) {
		t.Fatal("read token was persisted")
	}
}

func TestCreateCloudUserRejectsOversizedResponse(t *testing.T) {
	originalCloudURL := CLOUD_URL
	defer func() { CLOUD_URL = originalCloudURL }()
	cloud := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, strings.Repeat("x", maxCloudTokenResponseBytes+1))
	}))
	defer cloud.Close()
	CLOUD_URL = cloud.URL

	ws, _ := newTestWebServer()
	ws.httpClient = cloud.Client()
	app := fiber.New()
	ws.setupAPIRoutes(app)
	response, err := app.Test(httptest.NewRequest(http.MethodPost, "/con/create", nil))
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != fiber.StatusBadGateway {
		t.Fatalf("status = %d, want 502", response.StatusCode)
	}
}

func TestCreateCloudUserRejectsUnsafeDashboardURL(t *testing.T) {
	originalCloudURL := CLOUD_URL
	defer func() { CLOUD_URL = originalCloudURL }()
	cloud := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(CloudTokenResponse{
			ReadToken:    validCloudToken(),
			WriteToken:   validCloudToken(),
			DashboardURL: "https://user:password@cloud.example/",
		})
	}))
	defer cloud.Close()
	CLOUD_URL = cloud.URL

	ws, _ := newTestWebServer()
	ws.httpClient = cloud.Client()
	app := fiber.New()
	ws.setupAPIRoutes(app)
	response, err := app.Test(httptest.NewRequest(http.MethodPost, "/con/create", nil))
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != fiber.StatusBadGateway {
		t.Fatalf("status = %d, want 502", response.StatusCode)
	}
}

func TestCreateCloudUserDoesNotLogCloudResponseBody(t *testing.T) {
	originalCloudURL := CLOUD_URL
	defer func() { CLOUD_URL = originalCloudURL }()
	const secretBody = "secret-response-body"
	cloud := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = io.WriteString(w, secretBody)
	}))
	defer cloud.Close()
	CLOUD_URL = cloud.URL

	originalLogOutput := log.Writer()
	var logs bytes.Buffer
	log.SetOutput(&logs)
	defer log.SetOutput(originalLogOutput)

	ws, _ := newTestWebServer()
	ws.httpClient = cloud.Client()
	app := fiber.New()
	ws.setupAPIRoutes(app)
	response, err := app.Test(httptest.NewRequest(http.MethodPost, "/con/create", nil))
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if strings.Contains(logs.String(), secretBody) {
		t.Fatalf("cloud response body leaked into logs: %s", logs.String())
	}
}

func TestBrowserURLUsesReachableHost(t *testing.T) {
	tests := []struct {
		host string
		want string
	}{
		{host: "127.0.0.1", want: "http://127.0.0.1:8000"},
		{host: "0.0.0.0", want: "http://localhost:8000"},
		{host: "::", want: "http://localhost:8000"},
		{host: "::1", want: "http://[::1]:8000"},
	}
	for _, test := range tests {
		if got := browserURL(test.host, "8000"); got != test.want {
			t.Errorf("browserURL(%q) = %q, want %q", test.host, got, test.want)
		}
	}
}

func TestEmbeddedFrontendIsServed(t *testing.T) {
	ws, _ := newTestWebServer()
	app := fiber.New()
	ws.setupRoutes(app)

	indexResponse, err := app.Test(httptest.NewRequest("GET", "http://127.0.0.1/", nil))
	if err != nil {
		t.Fatal(err)
	}
	defer indexResponse.Body.Close()
	indexBody, err := io.ReadAll(indexResponse.Body)
	if err != nil {
		t.Fatal(err)
	}
	if indexResponse.StatusCode != fiber.StatusOK || !strings.Contains(string(indexBody), `<div id="root"></div>`) {
		t.Fatalf("embedded index returned %d and body %q", indexResponse.StatusCode, indexBody)
	}
	requireSecurityHeaders(t, indexResponse)

	assetMatch := regexp.MustCompile(`src="(/static/[^"]+\.js)"`).FindSubmatch(indexBody)
	if len(assetMatch) != 2 {
		t.Fatalf("embedded index does not reference a JavaScript bundle: %q", indexBody)
	}
	assetResponse, err := app.Test(httptest.NewRequest("GET", "http://127.0.0.1"+string(assetMatch[1]), nil))
	if err != nil {
		t.Fatal(err)
	}
	defer assetResponse.Body.Close()
	if assetResponse.StatusCode != fiber.StatusOK {
		t.Fatalf("embedded JavaScript bundle returned %d", assetResponse.StatusCode)
	}
}

func TestWebserverReturnsListenerError(t *testing.T) {
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()

	port := strconv.Itoa(listener.Addr().(*net.TCPAddr).Port)
	ws, mu := newTestWebServer()

	err = webserver(
		context.Background(),
		ws.settings,
		mu,
		ws.serChan,
		ws.active,
		ws.temp,
		"127.0.0.1",
		port,
		nil,
	)
	if err == nil {
		t.Fatal("webserver succeeded while its port was already occupied")
	}
	if !strings.Contains(err.Error(), "listen on HTTP address") {
		t.Fatalf("unexpected listener error: %v", err)
	}
}

func TestWebserverStopsCleanlyWhenContextIsCancelled(t *testing.T) {
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := strconv.Itoa(listener.Addr().(*net.TCPAddr).Port)
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	ws, mu := newTestWebServer()
	if err := webserver(
		ctx,
		ws.settings,
		mu,
		ws.serChan,
		ws.active,
		ws.temp,
		"127.0.0.1",
		port,
		nil,
	); err != nil {
		t.Fatalf("webserver did not shut down cleanly: %v", err)
	}
}

func TestWebserverIgnoresBrowserLaunchFailure(t *testing.T) {
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := strconv.Itoa(listener.Addr().(*net.TCPAddr).Port)
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	var openedURL string
	openBrowser := func(url string) error {
		openedURL = url
		return errors.New("browser unavailable")
	}

	ws, mu := newTestWebServer()
	if err := webserver(
		ctx,
		ws.settings,
		mu,
		ws.serChan,
		ws.active,
		ws.temp,
		"0.0.0.0",
		port,
		openBrowser,
	); err != nil {
		t.Fatalf("browser launch failure stopped the web server: %v", err)
	}

	wantURL := "http://localhost:" + port
	if openedURL != wantURL {
		t.Fatalf("opened URL = %q, want %q", openedURL, wantURL)
	}
}
