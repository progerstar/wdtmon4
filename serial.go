package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"strings"
	"time"

	"go.bug.st/serial"
	"go.bug.st/serial/enumerator"
)

const (
	WATCHDOG_USB_VID uint16 = 0x0483
	WATCHDOG_USB_PID uint16 = 0xa26d
)

type serialRequest struct {
	command  string
	response chan<- string
}

func (request serialRequest) respond(response string) {
	select {
	case request.response <- response:
	default:
	}
}

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

func findUSBSerialPort(ports []*enumerator.PortDetails, vid, pid uint16) (string, error) {
	wantVID := fmt.Sprintf("%04X", vid)
	wantPID := fmt.Sprintf("%04X", pid)

	for _, port := range ports {
		if port != nil && port.Name != "" && port.IsUSB &&
			strings.EqualFold(port.VID, wantVID) &&
			strings.EqualFold(port.PID, wantPID) {
			return port.Name, nil
		}
	}

	return "", fmt.Errorf("USB serial port %s:%s not found", wantVID, wantPID)
}

func resolveSerialPort(portName string) (string, error) {
	if portName != "" {
		return portName, nil
	}

	ports, err := enumerator.GetDetailedPortsList()
	if err != nil {
		return "", fmt.Errorf("enumerate serial ports: %w", err)
	}
	return findUSBSerialPort(ports, WATCHDOG_USB_VID, WATCHDOG_USB_PID)
}

func validateSerialWrite(written, expected int, err error) error {
	if err != nil {
		return err
	}
	if written != expected {
		return io.ErrShortWrite
	}
	return nil
}

func normalizeSerialResponse(response string) string {
	return response
}

func serialWorker(ctx context.Context, portName string, ch <-chan serialRequest) {
	if portName == "" {
		log.Printf("Serial worker started with USB auto-detection for %04X:%04X", WATCHDOG_USB_VID, WATCHDOG_USB_PID)
	} else {
		log.Printf("Serial worker started for port: %s", portName)
	}
	defer log.Println("Serial worker stopped")

	for {
		select {
		case <-ctx.Done():
			log.Println("Serial worker shutting down...")
			return
		case request, ok := <-ch:
			if !ok {
				return
			}
			cmd := request.command
			response := ""

			resolvedPortName, err := resolveSerialPort(portName)
			if err != nil {
				log.Printf("Failed to resolve serial port: %v", err)
				request.respond("")
				continue
			}

			port, err := serial.Open(resolvedPortName, &serial.Mode{BaudRate: 115200})
			if err != nil {
				log.Printf("Failed to open serial port %s: %v", resolvedPortName, err)
				request.respond("")
				continue
			}

			if err := port.SetReadTimeout(1 * time.Second); err != nil {
				log.Printf("Failed to set read timeout: %v", err)
				port.Close()
				request.respond("")
				continue
			}

			written, writeErr := port.Write([]byte(cmd))
			if err := validateSerialWrite(written, len(cmd), writeErr); err != nil {
				log.Printf("Failed to write command '%s': %v", cmd, err)
			} else {
				response = normalizeSerialResponse(ReadLine(port, 1*time.Second))
			}

			if closeErr := port.Close(); closeErr != nil {
				log.Printf("Failed to close serial port: %v", closeErr)
			}

			request.respond(response)
		}
	}
}
