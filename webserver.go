package main

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/datumbrain/nulltypes"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/filesystem"
	"github.com/gofiber/fiber/v2/middleware/monitor"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/google/uuid"
	"github.com/keybase/go-ps"
)

//go:embed web/build/*
var embedDirIndex embed.FS

//go:embed web/build/static/*
var embedDirStatic embed.FS

var startTime = time.Now()

func uptime() time.Duration {
	return time.Since(startTime)
}

type WebServer struct {
	settings      *Settings
	serChan       chan string
	active        *bool
	temp          *nulltypes.NullFloat64
	settingsMutex sync.RWMutex
	httpClient    *http.Client
}

func NewWebServer(settings *Settings, serChan chan string, active *bool, temp *nulltypes.NullFloat64) *WebServer {
	return &WebServer{
		settings:   settings,
		serChan:    serChan,
		active:     active,
		temp:       temp,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

func (ws *WebServer) validateSettings(sett *Settings) error {
	if sett.ConAlertTimeout < 1 || sett.ConAlertTimeout > 3600 {
		return fmt.Errorf("alert timeout must be between 1 and 3600 seconds")
	}
	if sett.ConAlertVal < 0 || sett.ConAlertVal > 100 {
		return fmt.Errorf("alert value must be between 0 and 100")
	}
	return nil
}

func (ws *WebServer) setupRoutes(app *fiber.App) {
	// Middleware
	app.Use(recover.New())
	app.Use(cors.New(cors.Config{
		AllowOrigins: "*",
		AllowMethods: "GET,POST,HEAD,PUT,DELETE,PATCH",
		AllowHeaders: "Origin,Content-Type,Accept,Authorization,id",
	}))

	// API routes (original paths)
	ws.setupAPIRoutes(app)

	// Monitoring
	app.Get("/monitor", monitor.New(monitor.Config{Title: "WDTMon4 Monitor"}))

	// Static files
	app.Use("/", filesystem.New(filesystem.Config{
		Root:       http.FS(embedDirIndex),
		PathPrefix: "web/build",
		Browse:     false,
	}))
	app.Use("/static", filesystem.New(filesystem.Config{
		Root:       http.FS(embedDirStatic),
		PathPrefix: "web/build/static",
		Browse:     false,
	}))
}

func (ws *WebServer) setupAPIRoutes(app *fiber.App) {
	// Settings
	app.Get("/settings", ws.getSettings)
	app.Post("/settings", ws.updateSettings)

	// Commands
	app.Get("/cmd/:cmd", ws.executeCommand)

	// Process list
	app.Get("/proc", ws.getProcessList)

	// Cloud connection
	con := app.Group("/con")
	con.Get("/user", ws.getCloudUser)
	con.Post("/create", ws.createCloudUser)

	// System info
	app.Get("/uptime", ws.getUptime)
	app.Get("/status", ws.getStatus)
}

func (ws *WebServer) getSettings(c *fiber.Ctx) error {
	ws.settingsMutex.RLock()
	defer ws.settingsMutex.RUnlock()
	return c.JSON(ws.settings)
}

func (ws *WebServer) updateSettings(c *fiber.Ctx) error {
	var newSettings Settings
	if err := c.BodyParser(&newSettings); err != nil {
		log.Printf("Failed to parse settings: %v", err)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid settings format",
		})
	}

	log.Printf("Updating settings: NetEn=%v, Net=%s, ProcEn=%v, Proc=%s, Pause=%v",
		newSettings.NetEn, newSettings.Net, newSettings.ProcEn, newSettings.Proc, newSettings.Pause)

	if err := ws.validateSettings(&newSettings); err != nil {
		log.Printf("Settings validation failed: %v", err)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	if err := newSettings.Write(); err != nil {
		log.Printf("Failed to write settings to file: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to save settings",
		})
	}

	ws.settingsMutex.Lock()
	defer ws.settingsMutex.Unlock()

	if err := ws.settings.Read(); err != nil {
		log.Printf("Failed to reload settings from file: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to reload settings",
		})
	}

	log.Printf("Settings updated successfully: NetEn=%v, Net=%s, ProcEn=%v, Proc=%s, Pause=%v",
		ws.settings.NetEn, ws.settings.Net, ws.settings.ProcEn, ws.settings.Proc, ws.settings.Pause)

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
		if *ws.active {
			return c.SendString("~A")
		}
		return c.SendStatus(fiber.StatusBadRequest)
	case "~G":
		if ws.temp.Valid {
			return c.SendString(fmt.Sprintf("%.1f", ws.temp.Float64))
		}
		return c.SendStatus(fiber.StatusBadRequest)
	default:
		res, err := sendrecv(cmd, ws.serChan)
		if err != nil {
			log.Printf("Command %s failed: %v", cmd, err)
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

func (ws *WebServer) getCloudUser(c *fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", CLOUD_URL+"/user", nil)
	if err != nil {
		log.Printf("Failed to create request: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to create request",
		})
	}

	userID := c.Get("id")
	if userID == "" {
		ws.settingsMutex.RLock()
		userID = ws.settings.ConUID
		ws.settingsMutex.RUnlock()
	} else {
		if _, err := uuid.Parse(userID); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Invalid user ID format",
			})
		}
		// Update settings with new user ID
		ws.settingsMutex.Lock()
		ws.settings.ConEn = true
		ws.settings.ConUID = userID
		if err := ws.settings.Write(); err != nil {
			log.Printf("Failed to save settings: %v", err)
		}
		ws.settingsMutex.Unlock()
	}

	req.Header.Set("id", userID)

	resp, err := ws.httpClient.Do(req)
	if err != nil {
		log.Printf("HTTP request failed: %v", err)
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{
			"error": "Cloud service unavailable",
		})
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Failed to read response: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to read response",
		})
	}

	return c.Send(body)
}

func (ws *WebServer) createCloudUser(c *fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", CLOUD_URL+"/user/create", nil)
	if err != nil {
		log.Printf("Failed to create request: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to create request",
		})
	}

	resp, err := ws.httpClient.Do(req)
	if err != nil {
		log.Printf("HTTP request failed: %v", err)
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{
			"error": "Cloud service unavailable",
		})
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Failed to read response: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to read response",
		})
	}

	var user struct {
		Id string `json:"id"`
	}
	if err := json.Unmarshal(body, &user); err == nil && user.Id != "" {
		ws.settingsMutex.Lock()
		ws.settings.ConUID = user.Id
		ws.settings.ConEn = true
		if err := ws.settings.Write(); err != nil {
			log.Printf("Failed to save settings: %v", err)
		}
		ws.settingsMutex.Unlock()
	}

	return c.Send(body)
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

func webserver(ctx context.Context, settings *Settings, serChan chan string, active *bool, temp *nulltypes.NullFloat64, hport string) {
	ws := NewWebServer(settings, serChan, active, temp)

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

	log.Printf("Web server started on http://localhost:%s", hport)

	go func() {
		if err := app.Listen(":" + hport); err != nil {
			log.Printf("Server failed to start: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("Web server shutting down...")

	// Graceful shutdown
	if err := app.Shutdown(); err != nil {
		log.Printf("Server shutdown error: %v", err)
	}
	log.Println("Web server stopped")
}
