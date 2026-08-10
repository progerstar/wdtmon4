package main

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/datumbrain/nulltypes"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/filesystem"
	"github.com/gofiber/fiber/v2/middleware/monitor"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/google/uuid"
	"github.com/keybase/go-ps"
)

//go:embed web/build/*
var embedDirIndex embed.FS

var startTime = time.Now()

const maxCloudTokenResponseBytes = 64 * 1024

func uptime() time.Duration {
	return time.Since(startTime)
}

type WebServer struct {
	settings      *Settings
	serChan       chan<- serialRequest
	active        *bool
	temp          *nulltypes.NullFloat64
	settingsMutex *sync.RWMutex
	httpClient    *http.Client
	bindHost      string
}

type readinessListener struct {
	net.Listener
	ready chan struct{}
	once  sync.Once
}

type cloudTokenCreateResponse struct {
	CloudTokenResponse
	DeviceID string `json:"device_id"`
}

func (listener *readinessListener) Accept() (net.Conn, error) {
	listener.once.Do(func() {
		close(listener.ready)
	})
	return listener.Listener.Accept()
}

func NewWebServer(settings *Settings, mu *sync.RWMutex, serChan chan<- serialRequest, active *bool, temp *nulltypes.NullFloat64, bindHost string) *WebServer {
	return &WebServer{
		settings:      settings,
		serChan:       serChan,
		active:        active,
		temp:          temp,
		settingsMutex: mu,
		httpClient:    &http.Client{Timeout: 10 * time.Second},
		bindHost:      bindHost,
	}
}

func canonicalAuthority(authority, scheme string) (string, error) {
	parsed, err := url.Parse(scheme + "://" + authority)
	if err != nil || parsed.Host == "" || parsed.User != nil || parsed.Path != "" || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", errors.New("invalid authority")
	}

	host := strings.ToLower(strings.TrimSuffix(parsed.Hostname(), "."))
	if host == "" {
		return "", errors.New("empty host")
	}
	if ip := net.ParseIP(host); ip != nil {
		host = ip.String()
	}
	port := parsed.Port()
	if (strings.EqualFold(scheme, "http") && port == "80") || (strings.EqualFold(scheme, "https") && port == "443") {
		port = ""
	}
	if port != "" {
		return net.JoinHostPort(host, port), nil
	}
	return host, nil
}

func originMatchesHost(origin, requestHost string) bool {
	parsed, err := url.Parse(origin)
	if err != nil || parsed.Host == "" || parsed.User != nil ||
		(!strings.EqualFold(parsed.Scheme, "http") && !strings.EqualFold(parsed.Scheme, "https")) ||
		(parsed.Path != "" && parsed.Path != "/") || parsed.RawQuery != "" || parsed.Fragment != "" {
		return false
	}

	originAuthority, err := canonicalAuthority(parsed.Host, parsed.Scheme)
	if err != nil {
		return false
	}
	hostAuthority, err := canonicalAuthority(requestHost, parsed.Scheme)
	return err == nil && originAuthority == hostAuthority
}

func authorityHostname(authority string) string {
	parsed, err := url.Parse("http://" + authority)
	if err != nil || parsed.Host == "" || parsed.User != nil || parsed.Path != "" || parsed.RawQuery != "" || parsed.Fragment != "" {
		return ""
	}
	return strings.TrimSuffix(parsed.Hostname(), ".")
}

func isLoopbackHost(host string) bool {
	host = strings.TrimSpace(strings.Trim(host, "[]"))
	if strings.EqualFold(strings.TrimSuffix(host, "."), "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func (ws *WebServer) requestSecurity(c *fiber.Ctx) error {
	c.Set(fiber.HeaderContentSecurityPolicy, "frame-ancestors 'none'")
	c.Set(fiber.HeaderXFrameOptions, "DENY")
	c.Set(fiber.HeaderXContentTypeOptions, "nosniff")
	c.Set(fiber.HeaderReferrerPolicy, "no-referrer")

	requestHost := string(c.Context().Host())
	if isLoopbackHost(ws.bindHost) && !isLoopbackHost(authorityHostname(requestHost)) {
		return c.SendStatus(fiber.StatusForbidden)
	}
	if origin := c.Get(fiber.HeaderOrigin); origin != "" && !originMatchesHost(origin, requestHost) {
		return c.SendStatus(fiber.StatusForbidden)
	}
	return c.Next()
}

func (ws *WebServer) validateSettings(sett *Settings) error {
	if sett.NetEn {
		if _, err := networkProbeAddress(sett.Net); err != nil {
			return fmt.Errorf("invalid network monitoring target: %w", err)
		}
	}
	if !isCloudDeviceID(sett.ConDev) {
		return fmt.Errorf("device id must be 1-64 characters: A-Z, a-z, 0-9, _ or -")
	}
	if sett.ConWriteToken != "" && !isCloudToken(sett.ConWriteToken) {
		return fmt.Errorf("invalid write token")
	}
	return nil
}

func (ws *WebServer) setupRoutes(app *fiber.App) {
	app.Use(recover.New())
	app.Use(ws.requestSecurity)

	ws.setupAPIRoutes(app)

	app.Get("/monitor", monitor.New(monitor.Config{Title: "WDTMon4 Monitor"}))

	app.Use("/", filesystem.New(filesystem.Config{
		Root:       http.FS(embedDirIndex),
		PathPrefix: "web/build",
	}))
}

func (ws *WebServer) setupAPIRoutes(app *fiber.App) {
	app.Get("/settings", ws.getSettings)
	app.Post("/settings", ws.updateSettings)

	app.Post("/cmd/:cmd", ws.executeCommand)

	app.Get("/proc", ws.getProcessList)

	con := app.Group("/con")
	con.Post("/create", ws.createCloudUser)
	con.Post("/validate", ws.validateCloudToken)
	con.Post("/clear", ws.clearCloudCredentials)

	app.Get("/uptime", ws.getUptime)
	app.Get("/status", ws.getStatus)
}

func (ws *WebServer) getSettings(c *fiber.Ctx) error {
	c.Set(fiber.HeaderCacheControl, "no-store")
	ws.settingsMutex.RLock()
	settings := *ws.settings
	configured := cloudWriteToken(ws.settings) != ""
	ws.settingsMutex.RUnlock()

	return c.JSON(settingsResponse{
		Net:           settings.Net,
		NetEn:         settings.NetEn,
		Proc:          settings.Proc,
		ProcEn:        settings.ProcEn,
		Diode:         settings.Diode,
		Pause:         settings.Pause,
		ConEn:         settings.ConEn,
		ConConfigured: configured,
		ConDev:        settings.ConDev,
	})
}

func (ws *WebServer) updateSettings(c *fiber.Ctx) error {
	var newSettings Settings
	if err := c.BodyParser(&newSettings); err != nil {
		log.Printf("Failed to parse settings: %v", err)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid settings format",
		})
	}

	ws.settingsMutex.Lock()
	defer ws.settingsMutex.Unlock()
	newSettings.ConWriteToken = cloudWriteToken(ws.settings)
	newSettings.ConUID = ""

	if err := ws.validateSettings(&newSettings); err != nil {
		log.Printf("Settings validation failed: %v", err)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	if err := newSettings.Write(); err != nil {
		log.Printf("Failed to write settings to file: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to save settings",
		})
	}
	*ws.settings = newSettings

	return c.SendStatus(fiber.StatusOK)
}

func (ws *WebServer) executeCommand(c *fiber.Ctx) error {
	cmd := c.Params("cmd")
	if cmd == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Command is required",
		})
	}

	switch cmd {
	case "~U":
		ws.settingsMutex.RLock()
		isActive := *ws.active
		ws.settingsMutex.RUnlock()
		if isActive {
			return c.SendString("~A")
		}
		return c.SendStatus(fiber.StatusBadRequest)
	case "~G":
		ws.settingsMutex.RLock()
		t := *ws.temp
		ws.settingsMutex.RUnlock()
		if t.Valid {
			return c.SendString(fmt.Sprintf("%.1f", t.Float64))
		}
		return c.SendStatus(fiber.StatusBadRequest)
	default:
		res, err := sendrecv(context.Background(), cmd, ws.serChan)
		if err != nil {
			log.Printf("Command %s failed: %v", cmd, err)
			if errors.Is(err, context.DeadlineExceeded) {
				return c.SendStatus(fiber.StatusGatewayTimeout)
			}
			return c.SendStatus(fiber.StatusInternalServerError)
		}
		return c.SendString(res)
	}
}

func (ws *WebServer) getProcessList(c *fiber.Ctx) error {
	processList, err := ps.Processes()
	if err != nil {
		log.Printf("Failed to get process list: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to get process list",
		})
	}

	processes := make([]Proc, 0, len(processList))
	for _, proc := range processList {
		if name := proc.Executable(); name != "" {
			processes = append(processes, Proc{Name: name})
		}
	}

	return c.JSON(processes)
}

func (ws *WebServer) createCloudUser(c *fiber.Ctx) error {
	c.Set(fiber.HeaderCacheControl, "no-store")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", CLOUD_URL+"/token", nil)
	if err != nil {
		log.Printf("Failed to create request: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to create request",
		})
	}
	req.Header.Set("Accept", "application/json")

	resp, err := ws.httpClient.Do(req)
	if err != nil {
		log.Printf("HTTP request failed: %v", err)
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{
			"error": "Cloud service unavailable",
		})
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxCloudTokenResponseBytes+1))
	if err != nil {
		log.Printf("Failed to read response: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to read response",
		})
	}
	if len(body) > maxCloudTokenResponseBytes {
		log.Printf("Cloud token response exceeded %d bytes", maxCloudTokenResponseBytes)
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{
			"error": "Invalid cloud response",
		})
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		log.Printf("Cloud token request failed: %s", resp.Status)
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{
			"error": "Cloud service unavailable",
		})
	}

	var tokenPair CloudTokenResponse
	if err := json.Unmarshal(body, &tokenPair); err != nil {
		log.Printf("Failed to parse cloud token response: %v", err)
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{
			"error": "Invalid cloud response",
		})
	}
	if !isCloudToken(tokenPair.ReadToken) || !isCloudToken(tokenPair.WriteToken) {
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{
			"error": "Invalid cloud token response",
		})
	}
	if err := validateDashboardURL(tokenPair.DashboardURL); err != nil {
		log.Printf("Cloud returned an invalid dashboard URL: %v", err)
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{
			"error": "Invalid cloud response",
		})
	}

	ws.settingsMutex.Lock()
	newSettings := *ws.settings
	newSettings.ConWriteToken = tokenPair.WriteToken
	newSettings.ConUID = ""
	newSettings.ConEn = true
	if !isCloudDeviceID(newSettings.ConDev) {
		newSettings.ConDev = uuid.NewString()
	}
	if err := newSettings.Write(); err != nil {
		ws.settingsMutex.Unlock()
		log.Printf("Failed to save settings: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to save settings",
		})
	}
	*ws.settings = newSettings
	ws.settingsMutex.Unlock()

	return c.JSON(cloudTokenCreateResponse{
		CloudTokenResponse: tokenPair,
		DeviceID:           newSettings.ConDev,
	})
}

func validateDashboardURL(rawURL string) error {
	if rawURL == "" {
		return nil
	}
	if rawURL != strings.TrimSpace(rawURL) {
		return errors.New("dashboard URL contains surrounding whitespace")
	}
	parsed, err := url.Parse(rawURL)
	if err != nil || !parsed.IsAbs() || parsed.Host == "" || parsed.User != nil {
		return errors.New("dashboard URL must be absolute and must not contain user information")
	}
	if !strings.EqualFold(parsed.Scheme, "http") && !strings.EqualFold(parsed.Scheme, "https") {
		return errors.New("dashboard URL must use HTTP or HTTPS")
	}
	return nil
}

func (ws *WebServer) validateCloudToken(c *fiber.Ctx) error {
	var input struct {
		WriteToken string `json:"write_token"`
	}
	if err := c.BodyParser(&input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid token request",
		})
	}

	writeToken := strings.TrimSpace(input.WriteToken)
	if !isCloudToken(writeToken) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid write token",
		})
	}

	ws.settingsMutex.RLock()
	deviceID := ws.settings.ConDev
	isActive := *ws.active
	temperature := *ws.temp
	ws.settingsMutex.RUnlock()

	if deviceID == "" {
		deviceID = uuid.NewString()
	} else if !isCloudDeviceID(deviceID) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid cloud device ID",
		})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	state := currentCloudState(isActive, temperature)
	if err := consend(ctx, writeToken, cloudStateURL(deviceID), &state, ws.httpClient); err != nil {
		var responseError *cloudHTTPError
		if errors.As(err, &responseError) &&
			(responseError.StatusCode == http.StatusUnauthorized || responseError.StatusCode == http.StatusForbidden) {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Invalid write token",
			})
		}

		log.Printf("Cloud write token validation failed: %v", err)
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{
			"error": "Cloud service unavailable",
		})
	}

	ws.settingsMutex.Lock()
	newSettings := *ws.settings
	newSettings.ConEn = true
	newSettings.ConUID = ""
	newSettings.ConWriteToken = writeToken
	newSettings.ConDev = deviceID
	if err := newSettings.Write(); err != nil {
		ws.settingsMutex.Unlock()
		log.Printf("Failed to save validated cloud credentials: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to save settings",
		})
	}
	*ws.settings = newSettings
	ws.settingsMutex.Unlock()

	return c.JSON(fiber.Map{"device_id": deviceID})
}

func (ws *WebServer) clearCloudCredentials(c *fiber.Ctx) error {
	ws.settingsMutex.Lock()
	defer ws.settingsMutex.Unlock()

	newSettings := *ws.settings
	newSettings.ConEn = false
	newSettings.ConUID = ""
	newSettings.ConWriteToken = ""
	if err := newSettings.Write(); err != nil {
		log.Printf("Failed to clear cloud credentials: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to save settings",
		})
	}
	*ws.settings = newSettings
	return c.SendStatus(fiber.StatusNoContent)
}

func (ws *WebServer) getUptime(c *fiber.Ctx) error {
	return c.SendString(fmt.Sprintf("%.0f", uptime().Seconds()))
}

func (ws *WebServer) getStatus(c *fiber.Ctx) error {
	ws.settingsMutex.RLock()
	defer ws.settingsMutex.RUnlock()

	return c.JSON(fiber.Map{
		"active":      *ws.active,
		"temperature": ws.temp,
		"uptime":      int64(uptime().Seconds()),
		"version":     VERSION,
	})
}

func browserURL(bindHost, port string) string {
	host := strings.TrimSpace(strings.Trim(bindHost, "[]"))
	if ip := net.ParseIP(host); ip != nil && ip.IsUnspecified() {
		host = "localhost"
	}
	return "http://" + net.JoinHostPort(host, port)
}

func webserver(ctx context.Context, settings *Settings, mu *sync.RWMutex, serChan chan<- serialRequest, active *bool, temp *nulltypes.NullFloat64, host, hport string, browserOpener func(string) error) error {
	ws := NewWebServer(settings, mu, serChan, active, temp, host)

	app := fiber.New(fiber.Config{
		DisableStartupMessage: true,
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			code := fiber.StatusInternalServerError
			if e, ok := err.(*fiber.Error); ok {
				code = e.Code
			}
			return c.Status(code).JSON(fiber.Map{
				"error": err.Error(),
			})
		},
	})

	ws.setupRoutes(app)

	listenAddress := net.JoinHostPort(host, hport)
	listener, err := net.Listen("tcp", listenAddress)
	if err != nil {
		return fmt.Errorf("listen on HTTP address %s: %w", listenAddress, err)
	}

	serveErr := make(chan error, 1)
	listenerReady := make(chan struct{})
	go func() {
		serveErr <- app.Listener(&readinessListener{
			Listener: listener,
			ready:    listenerReady,
		})
	}()

	select {
	case <-listenerReady:
	case err := <-serveErr:
		if err == nil {
			return errors.New("web server stopped during startup")
		}
		return fmt.Errorf("start HTTP server: %w", err)
	}

	webURL := browserURL(host, hport)
	log.Printf("Web server started on %s", webURL)
	if browserOpener != nil {
		if err := browserOpener(webURL); err != nil {
			log.Printf("Failed to open web interface in a browser: %v", err)
		}
	}

	select {
	case err := <-serveErr:
		if err == nil {
			return errors.New("web server stopped unexpectedly")
		}
		return fmt.Errorf("serve HTTP: %w", err)
	case <-ctx.Done():
		log.Println("Web server shutting down...")
	}

	if err := app.Shutdown(); err != nil {
		return fmt.Errorf("shut down web server: %w", err)
	}
	if err := <-serveErr; err != nil {
		return fmt.Errorf("serve HTTP during shutdown: %w", err)
	}

	log.Println("Web server stopped")
	return nil
}
