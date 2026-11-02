package main

import (
	"context"
	"errors"
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

const (
	// serialResponseTimeout bounds the wait for a response; silent commands
	// take this long.
	serialResponseTimeout = time.Second
	// serialIdleTimeout ends a response once a complete line was followed by
	// silence, so multi-line answers such as "Alert\n~A\n" are read whole.
	serialIdleTimeout     = 50 * time.Millisecond
	maxSerialResponseSize = 256
)

// errNoResponse reports a command that was written completely but was not
// answered. ~T1-~T3 are accepted this way; the firmware only reports Busy or
// Invalid for them.
var errNoResponse = errors.New("no response from device")

// errPortUnavailable reports that the serial port could not be found or
// opened, for example while the device is unplugged. The serial worker logs
// it once per outage, so callers need not log it again.
var errPortUnavailable = errors.New("serial port unavailable")

type serialResult struct {
	response string
	err      error
}

type serialRequest struct {
	command  string
	response chan<- serialResult
}

func (request serialRequest) respond(result serialResult) {
	select {
	case request.response <- result:
	default:
	}
}

// readResponse collects every line of one device response. port.Read must
// return (0, nil) after serialIdleTimeout without data.
func readResponse(port io.Reader, timeout time.Duration) (string, error) {
	var response []byte
	chunk := make([]byte, 64)
	deadline := time.Now().Add(timeout)
	for {
		n, err := port.Read(chunk)
		response = append(response, chunk[:n]...)
		if err != nil {
			return normalizeSerialResponse(string(response)), err
		}
		complete := len(response) > 0 && response[len(response)-1] == '\n'
		if (n == 0 && complete) || len(response) >= maxSerialResponseSize || time.Now().After(deadline) {
			return normalizeSerialResponse(string(response)), nil
		}
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

// normalizeSerialResponse keeps all response lines, separated by "\n".
func normalizeSerialResponse(response string) string {
	return strings.TrimSpace(strings.ReplaceAll(response, "\r\n", "\n"))
}

func serialWorker(ctx context.Context, portName string, ch <-chan serialRequest) {
	if portName == "" {
		log.Printf("Serial worker started with USB auto-detection for %04X:%04X", WATCHDOG_USB_VID, WATCHDOG_USB_PID)
	} else {
		log.Printf("Serial worker started for port: %s", portName)
	}
	defer log.Println("Serial worker stopped")

	var portLog conditionLog

	for {
		select {
		case <-ctx.Done():
			log.Println("Serial worker shutting down...")
			return
		case request, ok := <-ch:
			if !ok {
				return
			}
			request.respond(exchangeCommand(portName, request.command, &portLog))
		}
	}
}

func exchangeCommand(portName, cmd string, portLog *conditionLog) serialResult {
	resolvedPortName, err := resolveSerialPort(portName)
	if err != nil {
		portLog.set("Failed to resolve serial port: %v", err)
		return serialResult{err: fmt.Errorf("%w: %v", errPortUnavailable, err)}
	}

	port, err := serial.Open(resolvedPortName, &serial.Mode{BaudRate: 115200})
	if err != nil {
		portLog.set("Failed to open serial port %s: %v", resolvedPortName, err)
		return serialResult{err: fmt.Errorf("%w: %v", errPortUnavailable, err)}
	}
	portLog.clear("Serial port %s is available again", resolvedPortName)
	defer func() {
		if closeErr := port.Close(); closeErr != nil {
			log.Printf("Failed to close serial port: %v", closeErr)
		}
	}()

	if err := port.SetReadTimeout(serialIdleTimeout); err != nil {
		log.Printf("Failed to set read timeout: %v", err)
		return serialResult{err: err}
	}
	// Drop stale bytes so they are not taken as the answer to this command.
	if err := port.ResetInputBuffer(); err != nil {
		log.Printf("Failed to reset serial input buffer: %v", err)
	}

	written, writeErr := port.Write([]byte(cmd))
	if err := validateSerialWrite(written, len(cmd), writeErr); err != nil {
		log.Printf("Failed to write command '%s': %v", cmd, err)
		return serialResult{err: err}
	}

	response, readErr := readResponse(port, serialResponseTimeout)
	if response == "" {
		// The device may drop off USB right after ~T2/~T3; the command was
		// still delivered, so report it as unanswered rather than unsent.
		if readErr != nil {
			return serialResult{err: fmt.Errorf("%w: %v", errNoResponse, readErr)}
		}
		return serialResult{err: errNoResponse}
	}
	if readErr != nil {
		log.Printf("Serial read for '%s' ended with an error: %v", cmd, readErr)
	}
	return serialResult{response: response}
}
