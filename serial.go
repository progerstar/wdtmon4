package main

import (
	"bufio"
	"context"
	"log"
	"time"

	"go.bug.st/serial"
)

func ReadLine(port serial.Port, timeout time.Duration) string {
	s := make(chan string, 1)

	go func() {
		defer close(s)
		line, err := bufio.NewReader(port).ReadString('\n')
		if err != nil {
			s <- ""
		} else {
			s <- line
		}
	}()

	select {
	case line := <-s:
		return line
	case <-time.After(timeout):
		return ""
	}
}

func serialWorker(ctx context.Context, portName string, ch chan string) {
	log.Printf("Serial worker started for port: %s", portName)
	defer log.Println("Serial worker stopped")

	for {
		select {
		case <-ctx.Done():
			log.Println("Serial worker shutting down...")
			return
		case cmd := <-ch:
			response := ""

			port, err := serial.Open(portName, &serial.Mode{BaudRate: 115200})
			if err != nil {
				log.Printf("Failed to open serial port %s: %v", portName, err)
				ch <- ""
				continue
			}

			if err := port.SetReadTimeout(1 * time.Second); err != nil {
				log.Printf("Failed to set read timeout: %v", err)
				port.Close()
				ch <- ""
				continue
			}

			_, err = port.Write([]byte(cmd))
			if err != nil {
				log.Printf("Failed to write command '%s': %v", cmd, err)
			} else {
				response = ReadLine(port, 1*time.Second)
			}

			if closeErr := port.Close(); closeErr != nil {
				log.Printf("Failed to close serial port: %v", closeErr)
			}

			ch <- response
		}
	}
}
