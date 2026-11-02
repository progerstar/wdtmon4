package main

import (
	"bytes"
	"context"
	"errors"
	"log"
	"strings"
	"sync"
	"testing"
	"time"
)

// syncBuffer collects log output written from several goroutines.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func captureLog(t *testing.T) *syncBuffer {
	t.Helper()
	logs := &syncBuffer{}
	originalOutput, originalFlags := log.Writer(), log.Flags()
	log.SetOutput(logs)
	log.SetFlags(0)
	t.Cleanup(func() {
		log.SetOutput(originalOutput)
		log.SetFlags(originalFlags)
	})
	return logs
}

func TestConditionLogReportsChangesOnce(t *testing.T) {
	logs := captureLog(t)
	var condition conditionLog

	condition.clear("recovered before failing")
	condition.set("failed: %s", "a")
	condition.set("failed: %s", "a")
	condition.set("failed: %s", "b")
	condition.clear("recovered")
	condition.clear("recovered")
	condition.set("failed: %s", "a")

	want := "failed: a\nfailed: b\nrecovered\nfailed: a\n"
	if got := logs.String(); got != want {
		t.Fatalf("logged %q, want %q", got, want)
	}
}

func TestSerialWorkerReportsMissingPortOnce(t *testing.T) {
	logs := captureLog(t)
	ctx, cancel := context.WithCancel(context.Background())
	requests := make(chan serialRequest)
	done := make(chan struct{})
	go func() {
		serialWorker(ctx, "/dev/wdtmon4-test-missing-port", requests)
		close(done)
	}()

	for range 3 {
		_, err := sendrecvWithTimeout(ctx, "~I", requests, time.Second)
		if !errors.Is(err, errPortUnavailable) {
			t.Fatalf("error = %v, want errPortUnavailable", err)
		}
	}
	cancel()
	<-done

	if got := strings.Count(logs.String(), "Failed to open serial port"); got != 1 {
		t.Fatalf("missing port logged %d times, want once:\n%s", got, logs.String())
	}
}

func TestPerioderLogsRepeatedConditionsOnce(t *testing.T) {
	logs := captureLog(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	settings := &Settings{Pause: true}
	var mu sync.RWMutex
	active := false
	temperature := invalidTemperature()
	requests := make(chan serialRequest)
	done := make(chan struct{})
	go func() {
		perioderWithIntervals(ctx, settings, &mu, requests, &active, &temperature, 5*time.Millisecond, time.Hour)
		close(done)
	}()
	time.Sleep(60 * time.Millisecond)

	// Resume while heartbeats fail: a missing port is reported by the serial
	// worker, a silent device once by the perioder.
	mu.Lock()
	settings.Pause = false
	mu.Unlock()
	for _, err := range []error{errPortUnavailable, errPortUnavailable, errNoResponse, errNoResponse, errNoResponse} {
		select {
		case request := <-requests:
			request.respond(serialResult{err: err})
		case <-time.After(time.Second):
			t.Fatal("perioder did not send a heartbeat after resuming")
		}
	}

	// Recover: the device answers again.
	for _, exchange := range []struct{ command, response string }{
		{command: "~U", response: "~A"},
		{command: "~G", response: "~GEEEE"},
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
	cancel()
	<-done

	output := logs.String()
	for message, want := range map[string]int{
		"Pause mode enabled":                         1,
		"Pause mode disabled":                        1,
		"Heartbeat failed":                           1,
		"Heartbeat acknowledged by the device again": 1,
		"Temperature read":                           0,
	} {
		if got := strings.Count(output, message); got != want {
			t.Errorf("%q logged %d times, want %d:\n%s", message, got, want, output)
		}
	}
}
