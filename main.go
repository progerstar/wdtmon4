package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/datumbrain/nulltypes"
	"github.com/google/uuid"
	"github.com/tgoncuoglu/argparse"
)

const (
	VERSION       = "1.2"
	SETTINGS_FILE = "settings.json"
	CLOUD_URL     = "https://connect.unitx.pro"
)

type App struct {
	settings *Settings
	active   bool
	temp     nulltypes.NullFloat64
	serChan  chan string
	ctx      context.Context
	cancel   context.CancelFunc
}

func NewApp() *App {
	ctx, cancel := context.WithCancel(context.Background())
	return &App{
		settings: NewSettings(),
		serChan:  make(chan string),
		ctx:      ctx,
		cancel:   cancel,
	}
}

func NewSettings() *Settings {
	return &Settings{
		Diode:           true,
		ConDev:          uuid.NewString(),
		ConAlias:        "WatchDog",
		ConAlertVal:     1,
		ConAlertTimeout: 5,
	}
}

func parseArgs() (string, bool, bool, bool, string, error) {
	params := argparse.NewParser("wdtmon4", "Advanced WDT monitor for OD USB Watchdog")

	portName := params.StringPositional("port", &argparse.Options{
		Help: "Serial port name",
	})

	webEn := params.Flag("w", "web", &argparse.Options{
		Help: "Enable local web server with interface",
	})

	cloud := params.Flag("c", "cloud", &argparse.Options{
		Help: "Enable cloud connection",
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

	err := params.Parse(os.Args)
	if err != nil {
		return "", false, false, false, "", err
	}

	if *portName == "" {
		return "", false, false, false, "", errors.New("port name is required")
	}

	return *portName, *webEn, *cloud, *ver, *hport, nil
}

func (app *App) initSettings() error {
	if err := app.settings.Read(); err != nil {
		log.Printf("Failed to read settings, creating new: %v", err)
		if err := app.settings.Write(); err != nil {
			return fmt.Errorf("failed to create settings file: %w", err)
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

func (app *App) run(portName string, webEn, cloudEn bool, hport string) error {

	app.setupSignalHandler()

	go serialWorker(app.ctx, portName, app.serChan)

	if webEn {
		go perioder(app.ctx, cloudEn, app.settings, app.serChan, &app.active, &app.temp)

		webserver(app.ctx, app.settings, app.serChan, &app.active, &app.temp, hport)
	} else {
		perioder(app.ctx, cloudEn, app.settings, app.serChan, &app.active, &app.temp)
	}

	return nil
}

func main() {
	portName, webEn, cloudEn, showVersion, hport, err := parseArgs()
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
	log.Printf("Port: %s, Web: %v, Cloud: %v, HTTP Port: %s",
		portName, webEn, cloudEn, hport)

	if err := app.run(portName, webEn, cloudEn, hport); err != nil {
		log.Fatalf("Application failed: %v", err)
	}
}
