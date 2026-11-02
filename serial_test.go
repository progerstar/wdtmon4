package main

import (
	"context"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/datumbrain/nulltypes"
	"go.bug.st/serial/enumerator"
)

func TestFindUSBSerialPortReturnsFirstVIDPIDMatch(t *testing.T) {
	ports := []*enumerator.PortDetails{
		{Name: "non-usb", VID: "1234", PID: "5678"},
		{Name: "first", IsUSB: true, VID: "1234", PID: "abcd"},
		{Name: "second", IsUSB: true, VID: "1234", PID: "ABCD"},
	}

	portName, err := findUSBSerialPort(ports, 0x1234, 0xABCD)
	if err != nil {
		t.Fatal(err)
	}
	if portName != "first" {
		t.Fatalf("portName = %q, want first", portName)
	}
}

func TestFindUSBSerialPortReportsNoMatch(t *testing.T) {
	_, err := findUSBSerialPort([]*enumerator.PortDetails{
		{Name: "other", IsUSB: true, VID: "1234", PID: "5678"},
	}, 0xAAAA, 0xBBBB)
	if err == nil {
		t.Fatal("findUSBSerialPort succeeded without a matching port")
	}
}

func TestResolveSerialPortKeepsExplicitPort(t *testing.T) {
	portName, err := resolveSerialPort("explicit-port")
	if err != nil {
		t.Fatal(err)
	}
	if portName != "explicit-port" {
		t.Fatalf("portName = %q, want explicit-port", portName)
	}
}

func TestValidateSerialWriteRejectsPartialWrite(t *testing.T) {
	if err := validateSerialWrite(2, 3, nil); !errors.Is(err, io.ErrShortWrite) {
		t.Fatalf("error = %v, want io.ErrShortWrite", err)
	}
	if err := validateSerialWrite(3, 3, nil); err != nil {
		t.Fatalf("complete write failed: %v", err)
	}
}

func TestNormalizeSerialResponseDoesNotFabricateAcknowledgement(t *testing.T) {
	if got := normalizeSerialResponse(""); got != "" {
		t.Fatalf("empty response = %q, want empty", got)
	}
	for _, response := range []string{"Busy\n", "Invalid\r\n"} {
		if got := normalizeSerialResponse(response); got != strings.TrimSpace(response) {
			t.Errorf("response %q changed to %q", response, got)
		}
	}
}

// fakeSerialPort serves chunks like a serial port with a read timeout: an
// empty chunk is a poll interval without data.
type fakeSerialPort struct {
	chunks []string
	err    error
}

func (port *fakeSerialPort) Read(p []byte) (int, error) {
	if len(port.chunks) == 0 {
		if port.err != nil {
			return 0, port.err
		}
		return 0, nil
	}
	chunk := port.chunks[0]
	port.chunks = port.chunks[1:]
	return copy(p, chunk), nil
}

func TestReadResponseKeepsEveryLine(t *testing.T) {
	tests := []struct {
		name   string
		chunks []string
		want   string
	}{
		{name: "alert and acknowledgement in one packet", chunks: []string{"Alert\n~A\n"}, want: "Alert\n~A"},
		{name: "lines split across reads", chunks: []string{"Al", "ert\n", "~A\n"}, want: "Alert\n~A"},
		{name: "line completed after a pause", chunks: []string{"~F12", "", "34\n"}, want: "~F1234"},
		{name: "silence", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := readResponse(&fakeSerialPort{chunks: tt.chunks}, 20*time.Millisecond)
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Fatalf("readResponse() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestReadResponseStopsAtCompleteLineFollowedBySilence(t *testing.T) {
	port := &fakeSerialPort{chunks: []string{"~A\n", "", "late\n"}}
	got, err := readResponse(port, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if got != "~A" {
		t.Fatalf("readResponse() = %q, want ~A", got)
	}
}

func TestReadResponseReportsReadErrors(t *testing.T) {
	disconnected := errors.New("port closed")
	got, err := readResponse(&fakeSerialPort{err: disconnected}, time.Second)
	if !errors.Is(err, disconnected) || got != "" {
		t.Fatalf("readResponse() = %q, %v; want empty response and %v", got, err, disconnected)
	}
}

func TestParseHeartbeatResponse(t *testing.T) {
	tests := []struct {
		response     string
		acknowledged bool
		alert        bool
	}{
		{response: "~A", acknowledged: true},
		{response: "Alert\n~A", acknowledged: true, alert: true},
		{response: "Alert", alert: true},
		{response: "Invalid command"},
		{response: ""},
	}

	for _, tt := range tests {
		acknowledged, alert := parseHeartbeatResponse(tt.response)
		if acknowledged != tt.acknowledged || alert != tt.alert {
			t.Errorf("parseHeartbeatResponse(%q) = %v, %v; want %v, %v",
				tt.response, acknowledged, alert, tt.acknowledged, tt.alert)
		}
	}
}

func TestSendrecvReceivesItsOwnResponse(t *testing.T) {
	requests := make(chan serialRequest)
	go func() {
		request := <-requests
		if request.command != "~U" {
			t.Errorf("command = %q, want ~U", request.command)
		}
		request.respond(serialResult{response: " ~A\n"})
	}()

	response, err := sendrecvWithTimeout(context.Background(), "~U", requests, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if response != "~A" {
		t.Fatalf("response = %q, want ~A", response)
	}
}

func TestSendrecvTimesOutWhileWaitingForResponse(t *testing.T) {
	requests := make(chan serialRequest)
	accepted := make(chan struct{})
	go func() {
		<-requests
		close(accepted)
	}()

	_, err := sendrecvWithTimeout(context.Background(), "~U", requests, 20*time.Millisecond)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("error = %v, want context deadline exceeded", err)
	}
	<-accepted
}

func TestPerioderInvalidatesTemperatureWhenHeartbeatFails(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	settings := &Settings{}
	var mu sync.RWMutex
	active := true
	temperature := nulltypes.NullFloat64{Float64: 42.5, Valid: true}
	requests := make(chan serialRequest)
	done := make(chan struct{})
	go func() {
		perioder(ctx, settings, &mu, requests, &active, &temperature)
		close(done)
	}()

	select {
	case request := <-requests:
		if request.command != "~U" {
			t.Fatalf("command = %q, want ~U", request.command)
		}
		request.respond(serialResult{err: errNoResponse})
	case <-time.After(3 * time.Second):
		t.Fatal("perioder did not issue a heartbeat request")
	}

	deadline := time.After(time.Second)
	for {
		mu.RLock()
		isActive := active
		currentTemperature := temperature
		mu.RUnlock()
		isTemperatureValid := currentTemperature.Valid
		if !isActive && !isTemperatureValid {
			break
		}

		select {
		case <-deadline:
			t.Fatalf("heartbeat failure left stale state: active=%v temperature=%+v", isActive, currentTemperature)
		case <-time.After(10 * time.Millisecond):
		}
	}

	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("perioder did not stop after context cancellation")
	}
}

func TestPerioderAcceptsHeartbeatWithRebootAlert(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	settings := &Settings{}
	var mu sync.RWMutex
	active := false
	temperature := invalidTemperature()
	requests := make(chan serialRequest)
	done := make(chan struct{})
	go func() {
		perioderWithIntervals(ctx, settings, &mu, requests, &active, &temperature, 10*time.Millisecond, time.Hour)
		close(done)
	}()

	for _, exchange := range []struct{ command, response string }{
		{command: "~U", response: "Alert\n~A"},
		{command: "~G", response: "~G0235"},
	} {
		select {
		case request := <-requests:
			if request.command != exchange.command {
				t.Fatalf("command = %q, want %q", request.command, exchange.command)
			}
			request.respond(serialResult{response: exchange.response})
		case <-time.After(time.Second):
			t.Fatalf("perioder did not send %s", exchange.command)
		}
	}

	deadline := time.After(time.Second)
	for {
		mu.RLock()
		isActive, currentTemperature := active, temperature
		mu.RUnlock()
		if isActive && currentTemperature.Valid && currentTemperature.Float64 == 23.5 {
			break
		}
		select {
		case <-deadline:
			t.Fatalf("heartbeat with reboot alert left active=%v temperature=%+v", isActive, currentTemperature)
		case <-time.After(10 * time.Millisecond):
		}
	}

	cancel()
	<-done
}
