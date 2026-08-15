package main

import (
	"context"
	"errors"
	"io"
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
	for _, response := range []string{"Busy\n", "Blocked\n", "Invalid\n"} {
		if got := normalizeSerialResponse(response); got != response {
			t.Errorf("response %q changed to %q", response, got)
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
		request.respond(" ~A\n")
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
		request.respond("")
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
