package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"unicode"

	"github.com/datumbrain/nulltypes"
	"github.com/google/uuid"
	"github.com/tgoncuoglu/argparse"
)

const (
	SETTINGS_FILE = "settings.json"
)

var VERSION = "1.3"

type App struct {
	settings *Settings
	mu       sync.RWMutex
	active   bool
	temp     nulltypes.NullFloat64
	serChan  chan serialRequest
	ctx      context.Context
	cancel   context.CancelFunc
}

func NewApp() *App {
	ctx, cancel := context.WithCancel(context.Background())
	return &App{
		settings: NewSettings(),
		serChan:  make(chan serialRequest),
		ctx:      ctx,
		cancel:   cancel,
	}
}

func NewSettings() *Settings {
	return &Settings{
		Diode:  true,
		ConDev: uuid.NewString(),
	}
}

func parseArgs() (string, bool, bool, bool, string, string, error) {
	params := argparse.NewParser("wdtmon4", "Advanced WDT monitor for OD USB Watchdog")

	portName := params.StringPositional("port", &argparse.Options{
		Help: "Serial port name (auto-detected by USB VID:PID when omitted)",
	})

	headless := params.Flag("", "headless", &argparse.Options{
		Help: "Do not open the local web interface in a browser",
	})
	params.Flag("w", "web", &argparse.Options{
		Help: "Enable the local web interface (enabled by default; retained for compatibility)",
	})

	cloud := params.Flag("c", "cloud", &argparse.Options{
		Help: "Enable cloud connection for this session",
	})

	ver := params.Flag("v", "version", &argparse.Options{
		Help: "Show version and exit",
	})

	hport := params.String("p", "hport", &argparse.Options{
		Help:    "HTTP port",
		Default: "8000",
		Validate: func(args []string) error {
			if len(args) == 0 {
				return errors.New("port is required")
			}
			n, err := strconv.ParseInt(args[0], 10, 32)
			if err != nil || n < 1 || n > 65535 {
				return errors.New("port must be a positive integer between 1 and 65535")
			}
			return nil
		},
	})
	host := params.String("", "host", &argparse.Options{
		Help:    "HTTP bind host",
		Default: "127.0.0.1",
		Validate: func(args []string) error {
			if len(args) == 0 || args[0] == "" {
				return errors.New("host is required")
			}
			if strings.TrimSpace(args[0]) != args[0] {
				return errors.New("host must not contain surrounding whitespace")
			}
			for _, ch := range args[0] {
				if unicode.IsControl(ch) {
					return errors.New("host must not contain control characters")
				}
			}
			return nil
		},
	})

	err := params.Parse(os.Args)
	if err != nil {
		return "", false, false, false, "", "", err
	}

	return *portName, *headless, *cloud, *ver, *host, *hport, nil
}

func openBrowser(url string) error {
	var command string
	var args []string

	switch runtime.GOOS {
	case "darwin":
		command = "open"
		args = []string{url}
	case "windows":
		command = "rundll32"
		args = []string{"url.dll,FileProtocolHandler", url}
	case "linux":
		command = "xdg-open"
		args = []string{url}
	default:
		return fmt.Errorf("opening a browser is not supported on %s", runtime.GOOS)
	}

	cmd := exec.Command(command, args...)
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() {
		_ = cmd.Wait()
	}()
	return nil
}

func (app *App) initSettings() error {
	if err := app.settings.Read(); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("failed to read settings file: %w", err)
		}

		log.Printf("Settings file does not exist, creating a new one")
		if err := app.settings.Write(); err != nil {
			return fmt.Errorf("failed to create settings file: %w", err)
		}
		return nil
	}

	if app.settings.ConDev == "" {
		app.settings.ConDev = uuid.NewString()
		if err := app.settings.Write(); err != nil {
			return fmt.Errorf("failed to migrate cloud device ID: %w", err)
		}
	}
	return nil
}

func (app *App) setupSignalHandler() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		log.Printf("Received signal: %v, shutting down...", sig)
		app.cancel()
	}()
}

func (app *App) applyCloudStartupOverride(enabled bool) {
	if !enabled {
		return
	}

	app.mu.Lock()
	app.settings.ConEn = true
	app.mu.Unlock()
}

func (app *App) run(portName string, headless, cloudEn bool, host, hport string) error {

	app.setupSignalHandler()
	// The CLI flag is a startup override. Afterwards the persisted/UI setting
	// remains authoritative, so the web interface can still disable cloud
	// delivery while the process is running.
	app.applyCloudStartupOverride(cloudEn)

	go serialWorker(app.ctx, portName, app.serChan)
	go perioder(app.ctx, app.settings, &app.mu, app.serChan, &app.active, &app.temp)

	var browserOpener func(string) error
	if !headless {
		browserOpener = openBrowser
	}
	if err := webserver(app.ctx, app.settings, &app.mu, app.serChan, &app.active, &app.temp, host, hport, browserOpener); err != nil {
		app.cancel()
		return err
	}

	return nil
}

func main() {
	portName, headless, cloudEn, showVersion, host, hport, err := parseArgs()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if showVersion {
		fmt.Printf("wdtmon4 v%s\n", VERSION)
		os.Exit(0)
	}

	defer log.Println("App stopped")

	app := NewApp()
	defer app.cancel()

	if err := app.initSettings(); err != nil {
		log.Fatalf("Failed to initialize settings: %v", err)
	}

	log.Printf("Starting wdtmon4 v%s", VERSION)
	if portName == "" {
		log.Printf("Port: auto (%04X:%04X), Headless: %v, Cloud: %v, HTTP: %s",
			WATCHDOG_USB_VID, WATCHDOG_USB_PID, headless, cloudEn, net.JoinHostPort(host, hport))
	} else {
		log.Printf("Port: %s, Headless: %v, Cloud: %v, HTTP: %s",
			portName, headless, cloudEn, net.JoinHostPort(host, hport))
	}

	if err := app.run(portName, headless, cloudEn, host, hport); err != nil {
		log.Fatalf("Application failed: %v", err)
	}
}
