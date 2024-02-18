package main

import (
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

func webserver(settings *Settings, ch chan string, active *bool, temp *nulltypes.NullFloat64, hport string) {
	log.Println("webserver started on http://localhost:" + hport)

	var SettingsMutex sync.Mutex

	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	app.Use(cors.New())

	app.Get("/monitor", monitor.New(monitor.Config{Title: "Monitor"}))

	app.Get("/settings", func(c *fiber.Ctx) error {
		return c.JSON(settings)
	})

	app.Post("/settings", func(c *fiber.Ctx) error {
		sett := new(Settings)
		if err := c.BodyParser(sett); err != nil {
			log.Println(err)
			return c.SendStatus(fiber.StatusInternalServerError)
		}
		err := sett.Write()
		if err != nil {
			return c.SendStatus(fiber.StatusInternalServerError)
		}
		SettingsMutex.Lock()
		settings.Read()
		SettingsMutex.Unlock()
		return c.SendStatus(fiber.StatusOK)
	})

	app.Get("/cmd/:cmd", func(c *fiber.Ctx) error {
		if c.Params("cmd") == "~U" {
			if *active {
				return c.SendString("~A")
			} else {
				return c.SendStatus(fiber.StatusBadRequest)
			}
		} else if c.Params("cmd") == "~G" {
			if temp.Valid {
				return c.SendString(fmt.Sprintf("%.1f", temp.Float64))
			} else {
				return c.SendStatus(fiber.StatusBadRequest)
			}
		}
		res := sendrecv(c.Params("cmd"), ch)
		return c.SendString(res)
	})

	app.Get("/proc", func(c *fiber.Ctx) error {
		processList, err := ps.Processes()
		if err != nil {
			log.Println("ps.Processes() Failed, are you using windows?")
			return c.SendStatus(fiber.StatusInternalServerError)
		}

		p := make([]Proc, len(processList))
		for x := range processList {
			p[x] = Proc{processList[x].Executable()}
		}
		return c.JSON(p)
	})

	app.Get("/con/user", func(c *fiber.Ctx) error {
		client := &http.Client{}
		req, _ := http.NewRequest("GET", CLOUD_URL+"/user", nil)
		id, err := uuid.Parse(c.Get("id"))

		if err != nil {
			req.Header.Set("id", settings.ConUID)
		} else {
			req.Header.Set("id", id.String())
			SettingsMutex.Lock()
			settings.ConEn = true
			settings.ConUID = id.String()
			settings.Write()
			SettingsMutex.Unlock()
		}
		resp, err := client.Do(req)
		if err != nil {
			return c.SendStatus(fiber.StatusInternalServerError)
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return c.SendStatus(fiber.StatusInternalServerError)
		}
		return c.Send(body)
	})

	app.Post("/con/create", func(c *fiber.Ctx) error {
		client := &http.Client{}
		req, _ := http.NewRequest("POST", CLOUD_URL+"/user/create", nil)
		resp, err := client.Do(req)
		if err != nil {
			return c.SendStatus(fiber.StatusInternalServerError)
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return c.SendStatus(fiber.StatusInternalServerError)
		}
		user := struct {
			Id string `json:"id"`
		}{}

		if json.Unmarshal(body, &user) == nil {
			SettingsMutex.Lock()
			settings.ConUID = user.Id
			settings.Write()
			SettingsMutex.Unlock()
		}
		return c.Send(body)
	})

	app.Get("/uptime", func(c *fiber.Ctx) error {
		return c.SendString(fmt.Sprintf("%.0f", uptime().Seconds()))
	})

	app.Use("/", filesystem.New(filesystem.Config{
		Root:       http.FS(embedDirIndex),
		PathPrefix: "web/build",
		Browse:     false,
	}))

	app.Use("/static", filesystem.New(filesystem.Config{
		Root:       http.FS(embedDirStatic),
		PathPrefix: "./web/build/static",
		Browse:     false,
	}))

	log.Println(app.Listen(":" + hport))
}
