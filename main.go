package main

import (
	"fmt"
	"log"
	"os"

	"github.com/google/uuid"
	"github.com/tgoncuoglu/argparse"
)

const (
	VERSION       = "0.2"
	SETTINGS_FILE = "settings.json"
	CLOUD_URL     = "https://connect.unitx.pro"
	PORT          = ":8000"
)

func NewSettings() Settings {
	return Settings{Diode: true, ConDev: uuid.NewString(), ConAlias: "WatchDog", ConAlertVal: 1, ConAlertTimeout: 5}
}

func main() {
	var t = NullFloat64{Valid: false}
	active := false
	ser := make(chan string)

	params := argparse.NewParser("wdtmon4", "Advanced WDT monitor for OD USB Watchdog")
	portName := params.StringPositional("port", &argparse.Options{Help: "Serial port name"})
	webEn := params.Flag("w", "web", &argparse.Options{Help: "Enable local web server with interface"})
	cloud := params.Flag("c", "cloud", &argparse.Options{Help: "Enable cloud connection"})
	ver := params.Flag("v", "version", &argparse.Options{Help: "Show version and exit"})

	err := params.Parse(os.Args)
	if (err != nil) || (*portName == "") {
		fmt.Print(params.Usage(err))
		os.Exit(1)
	}

	if *ver {
		fmt.Println("wdtmon4 v" + VERSION)
		os.Exit(0)
	}

	defer func() {
		log.Println("App is stopped")
	}()

	settings := NewSettings()
	if err := settings.Read(); err != nil {
		if err := settings.Write(); err != nil {
			log.Fatalln("Setting file creation error. Exit.")
		}
	}

	go serialWorker(*portName, ser)

	if *webEn {
		go perioder(*cloud, &settings, ser, &active, &t)
		webserver(&settings, ser, &active, &t)
	} else {
		perioder(*cloud, &settings, ser, &active, &t)
	}

}
