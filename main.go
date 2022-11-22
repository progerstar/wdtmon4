package main

import (
	"bufio"
	"bytes"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/filesystem"
	"github.com/gofiber/fiber/v2/middleware/monitor"
	"github.com/google/uuid"
	"github.com/keybase/go-ps"

	"go.bug.st/serial"
)

const SETTINGS_FILE = "settings.json"
const CLOUD_URL = "https://connect.unitx.pro"
const PORT = ":8000"

//go:embed web/build/*
var embedDirIndex embed.FS

//go:embed web/build/static/*
var embedDirStatic embed.FS

var startTime = time.Now()

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

func NewSettings() Settings {
	return Settings{Diode: true, ConDev: uuid.NewString(), ConAlias: "WatchDog", ConAlertVal: 1, ConAlertTimeout: 5}
}

func uptime() time.Duration {
	return time.Since(startTime)
}

func ping(host string) error {
	seconds := 5
	timeOut := time.Duration(seconds) * time.Second
	conn, err := net.DialTimeout("tcp", host+":80", timeOut)
	if err != nil {
		conn.Close()
	}
	//fmt.Printf("Remote Address : %s \n", conn.RemoteAddr().String())
	//fmt.Printf("Local Address : %s \n", conn.LocalAddr().String())
	return err
}

func perioder(settings *Settings, portName string, mutex *sync.Mutex, active *bool, temp *NullFloat64) {
	heartbeat := time.Tick(2 * time.Second)
	connbeat := time.Tick(1 * time.Minute)
	confirst := true

	for {

		select {
		// ...
		case <-connbeat:
			if !*active {
				return
			}
			state := ConnectState{Type: 5, Value1: *temp, Value2: 2, Alias: settings.ConAlias}
			if confirst {
				confirst = false
			} else {
				state.Value2 = 1
			}

			if settings.ConEn && len(settings.ConUID) > 0 {
				client := &http.Client{}
				stateB, err := json.Marshal(state)

				if err == nil {
					//fmt.Println("send to connect", CLOUD_URL+"/state/"+settings.ConDev, stateB)
					req, _ := http.NewRequest("POST", CLOUD_URL+"/state/"+settings.ConDev, bytes.NewBuffer(stateB))
					req.Header.Add("Content-Type", "application/json")
					req.Header.Add("Content-Length", strconv.Itoa(len(stateB)))
					req.Header.Add("id", settings.ConUID)
					resp, err := client.Do(req)
					if err == nil {
						resp.Body.Close()
					}
				}
			}

		case <-heartbeat:
			res := true
			if settings.NetEn && (len(settings.Net) != 0) {
				if ping(settings.Net) != nil {
					res = false
					*active = false
				}
			}

			if res && settings.ProcEn && (len(settings.Proc) != 0) {
				if FindProcess(settings.Proc) != nil {
					res = false
					*active = false
				}
			}

			if res && settings.Pause {
				res = false
				*active = false
			}

			if res {
				*active = (sendrecv(portName, mutex, "~U") == "~A")
				tbuf := sendrecv(portName, mutex, "~G")
				if n, err := strconv.ParseFloat(tbuf[2:6], 64); err == nil {
					*temp = NullFloat64{n / 10, true}
				} else {
					*temp = NullFloat64{0, false}
				}

			}
		}
	}
}

func FindProcess(key string) error {
	err := errors.New("not found")
	ps, _ := ps.Processes()
	for i := range ps {
		if ps[i].Executable() == key {
			err = nil
			break
		}
	}
	return err
} // FindProcess( key string ) ( int, string, error )

func recvchecker(cmd string) string {
	switch cmd {
	case "~U":
		return "~A"
	case "~W":
		return "~F"
	default:
		return cmd
	}
}

func ReadLine(port serial.Port, timeout time.Duration) (string, error) {
	s := make(chan string)
	e := make(chan error)

	go func() {
		line, err := bufio.NewReader(port).ReadString('\n')
		if err != nil {
			e <- err
		} else {
			s <- line
		}
		close(s)
		close(e)
	}()

	select {
	case line := <-s:
		return line, nil
	case err := <-e:
		return "", err
	case <-time.After(timeout):
		return "", errors.New("Timeout")
	}
}

func sendrecv(portName string, mutex *sync.Mutex, cmd string) string {
	mutex.Lock()
	defer mutex.Unlock()

	port, err := serial.Open(portName, &serial.Mode{BaudRate: 115200})
	if err != nil {
		return err.Error()
	}
	defer port.Close()
	port.SetReadTimeout(1 * time.Second)

	_, err = port.Write([]byte(cmd))
	if err != nil {
		return err.Error()
	}

	res, err := ReadLine(port, 1*time.Second)
	if err != nil {
		return err.Error()
	}

	i := strings.Index(res, recvchecker(cmd[:2]))
	if i > -1 {
		return res[i:strings.Index(res[i:], "\n")]
	} else {
		return "Err"
	}
}

func main() {
	var SendMutex, SettingsMutex sync.Mutex
	var temp = NullFloat64{Valid: false}
	var portName string
	active := false

	settings := NewSettings()
	if err := settings.Read(); err != nil {
		if err := settings.Write(); err != nil {
			log.Println("New setting creation error. Exit.")
			return
		}
	}

	if len(os.Args) == 2 {
		portName = os.Args[1]
	} else {
		log.Println("wdtmon4 wrong parameters")
		fmt.Println("Usage: wdtmon4 device_port")
		return
	}

	log.Println("wdtmon4 started on http://localhost" + PORT)
	defer func() {
		log.Println("wdtmon4 stopped")
	}()

	go perioder(&settings, portName, &SendMutex, &active, &temp)

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
		SettingsMutex.Lock()
		settings = *sett
		err := settings.Write()
		SettingsMutex.Unlock()
		if err != nil {
			return c.SendStatus(fiber.StatusInternalServerError)
		}
		return c.SendStatus(fiber.StatusOK)
	})

	app.Get("/cmd/:cmd", func(c *fiber.Ctx) error {
		if c.Params("cmd") == "~U" {
			if active {
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
		res := sendrecv(portName, &SendMutex, c.Params("cmd"))
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

	log.Fatal(app.Listen(PORT))
}
