package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/datumbrain/nulltypes"
	"github.com/keybase/go-ps"
)

const (
	defaultTimeout       = 5 * time.Second
	serialCommandTimeout = 2 * time.Second
	heartbeatInterval    = 2 * time.Second
	connbeatInterval     = 5 * time.Minute
	maxCloudErrorBytes   = 64 * 1024
)

type cloudHTTPError struct {
	StatusCode int
	Status     string
	Code       string
}

func (err *cloudHTTPError) Error() string {
	if err.Code != "" {
		return fmt.Sprintf("cloud returned %s (%s)", err.Status, err.Code)
	}
	return "cloud returned " + err.Status
}

func validateTCPPort(port string) (string, error) {
	n, err := strconv.Atoi(port)
	if err != nil || n < 1 || n > 65535 {
		return "", fmt.Errorf("port must be an integer between 1 and 65535")
	}
	return strconv.Itoa(n), nil
}

func networkProbeAddress(target string) (string, error) {
	if strings.IndexFunc(target, func(r rune) bool {
		return r < 0x20 || r == 0x7f
	}) >= 0 {
		return "", errors.New("target contains control characters")
	}
	target = strings.TrimSpace(target)
	if target == "" {
		return "", errors.New("target is required")
	}

	if strings.Contains(target, "://") {
		parsedURL, err := url.ParseRequestURI(target)
		if err != nil {
			return "", fmt.Errorf("invalid URL: %w", err)
		}
		host := parsedURL.Hostname()
		if host == "" {
			return "", errors.New("URL hostname is required")
		}

		port := parsedURL.Port()
		if port == "" {
			switch strings.ToLower(parsedURL.Scheme) {
			case "http":
				port = "80"
			case "https":
				port = "443"
			default:
				return "", fmt.Errorf("URL scheme %q requires an explicit port", parsedURL.Scheme)
			}
		}
		port, err = validateTCPPort(port)
		if err != nil {
			return "", err
		}
		return net.JoinHostPort(host, port), nil
	}

	if host, port, err := net.SplitHostPort(target); err == nil {
		if host == "" {
			return "", errors.New("hostname is required")
		}
		port, err = validateTCPPort(port)
		if err != nil {
			return "", err
		}
		return net.JoinHostPort(host, port), nil
	}

	ipLiteral := strings.TrimSuffix(strings.TrimPrefix(target, "["), "]")
	if ip := net.ParseIP(ipLiteral); ip != nil {
		return net.JoinHostPort(ipLiteral, "80"), nil
	}

	if strings.Contains(target, ":") {
		return "", errors.New("target with a colon must use host:port syntax")
	}
	if strings.ContainsAny(target, "/?#@ ") {
		return "", errors.New("invalid hostname")
	}

	return net.JoinHostPort(target, "80"), nil
}

func ping(target string) error {
	address, err := networkProbeAddress(target)
	if err != nil {
		return err
	}

	conn, err := net.DialTimeout("tcp", address, defaultTimeout)
	if err != nil {
		return fmt.Errorf("connect to %s: %w", address, err)
	}
	defer conn.Close()
	return nil
}

func FindProcess(key string) (bool, error) {
	processes, err := ps.Processes()
	if err != nil {
		return false, err
	}

	for _, p := range processes {
		if p.Executable() == key {
			return true, nil
		}
	}
	return false, nil
}

func sendrecv(ctx context.Context, cmd string, ch chan<- serialRequest) (string, error) {
	return sendrecvWithTimeout(ctx, cmd, ch, serialCommandTimeout)
}

func sendrecvWithTimeout(ctx context.Context, cmd string, ch chan<- serialRequest, timeout time.Duration) (string, error) {
	requestCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	response := make(chan string, 1)
	request := serialRequest{command: cmd, response: response}

	select {
	case ch <- request:
	case <-requestCtx.Done():
		return "", fmt.Errorf("send serial command %q: %w", cmd, requestCtx.Err())
	}

	select {
	case res := <-response:
		if res == "" {
			return "", errors.New("empty response")
		}
		return strings.TrimSpace(res), nil
	case <-requestCtx.Done():
		return "", fmt.Errorf("wait for serial command %q: %w", cmd, requestCtx.Err())
	}
}

func consend(ctx context.Context, writeToken, url string, state *ConnectState, client *http.Client) error {
	if client == nil {
		client = &http.Client{Timeout: defaultTimeout}
	}
	writeToken = strings.TrimSpace(writeToken)
	if !isCloudToken(writeToken) {
		return errors.New("invalid cloud write token")
	}

	stateBytes, err := json.Marshal(state)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(stateBytes))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+writeToken)

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, maxCloudErrorBytes))
		if readErr != nil {
			return fmt.Errorf("read cloud error response: %w", readErr)
		}

		var responseError struct {
			Code string `json:"error"`
		}
		_ = json.Unmarshal(body, &responseError)
		return &cloudHTTPError{
			StatusCode: resp.StatusCode,
			Status:     resp.Status,
			Code:       responseError.Code,
		}
	}

	return nil
}

type cloudSendRequest struct {
	writeToken string
	url        string
	state      ConnectState
	initial    bool
}

type cloudSendWorker struct {
	ctx      context.Context
	cancel   context.CancelFunc
	client   *http.Client
	pending  chan cloudSendRequest
	finished chan struct{}
}

func newCloudSendWorker(ctx context.Context, client *http.Client) *cloudSendWorker {
	workerCtx, cancel := context.WithCancel(ctx)
	worker := &cloudSendWorker{
		ctx:      workerCtx,
		cancel:   cancel,
		client:   client,
		pending:  make(chan cloudSendRequest, 1),
		finished: make(chan struct{}),
	}
	go worker.run()
	return worker
}

func (worker *cloudSendWorker) run() {
	defer close(worker.finished)
	for {
		select {
		case <-worker.ctx.Done():
			return
		case request := <-worker.pending:
			err := consend(worker.ctx, request.writeToken, request.url, &request.state, worker.client)
			if err == nil || worker.ctx.Err() != nil {
				continue
			}
			if request.initial {
				log.Printf("Initial cloud send failed: %v", err)
			} else {
				log.Printf("Cloud send failed: %v", err)
			}
		}
	}
}

func (worker *cloudSendWorker) submit(request cloudSendRequest) {
	for {
		select {
		case <-worker.ctx.Done():
			return
		case worker.pending <- request:
			return
		default:
		}

		// Keep only the newest queued state.
		select {
		case <-worker.ctx.Done():
			return
		case <-worker.pending:
		default:
		}
	}
}

func (worker *cloudSendWorker) stop() {
	worker.cancel()
	<-worker.finished
}

func cloudEnabled(settings *Settings) bool {
	return settings != nil && settings.ConEn && isCloudDeviceID(settings.ConDev) && cloudWriteToken(settings) != ""
}

func cloudConnectionStatus(active bool) *int64 {
	status := int64(0)
	if active {
		status = 1
	}
	return &status
}

func currentCloudState(active bool, temp nulltypes.NullFloat64) ConnectState {
	return ConnectState{
		Type:   5,
		Value1: temp,
		Value2: cloudConnectionStatus(active),
	}
}

func invalidTemperature() nulltypes.NullFloat64 {
	return nulltypes.NullFloat64{Float64: 0, Valid: false}
}

func parseTemperature(tbuf string) nulltypes.NullFloat64 {
	if strings.Contains(tbuf, "EEEE") {
		return invalidTemperature()
	}

	if len(tbuf) >= 6 && strings.HasPrefix(tbuf, "~G") {
		tempStr := tbuf[2:6]
		if n, err := strconv.ParseFloat(tempStr, 64); err == nil {
			temp := n / 10
			return nulltypes.NullFloat64{Float64: temp, Valid: true}
		} else {
			log.Printf("parseTemperature: failed to parse temperature '%s': %v", tempStr, err)
		}
	}

	log.Printf("parseTemperature: invalid format '%s'", tbuf)
	return invalidTemperature()
}

func perioder(ctx context.Context, settings *Settings, mu *sync.RWMutex, ch chan<- serialRequest, active *bool, temp *nulltypes.NullFloat64) {
	perioderWithIntervals(ctx, settings, mu, ch, active, temp, heartbeatInterval, connbeatInterval)
}

func perioderWithIntervals(
	ctx context.Context,
	settings *Settings,
	mu *sync.RWMutex,
	ch chan<- serialRequest,
	active *bool,
	temp *nulltypes.NullFloat64,
	heartbeatEvery time.Duration,
	cloudEvery time.Duration,
) {
	log.Println("Perioder started")
	defer log.Println("Perioder stopped")

	client := &http.Client{Timeout: defaultTimeout}
	cloudWorker := newCloudSendWorker(ctx, client)
	defer cloudWorker.stop()

	readSettings := func() Settings { mu.RLock(); defer mu.RUnlock(); return *settings }
	readTemp := func() nulltypes.NullFloat64 { mu.RLock(); defer mu.RUnlock(); return *temp }
	readHealth := func() (bool, nulltypes.NullFloat64) {
		mu.RLock()
		defer mu.RUnlock()
		return *active, *temp
	}
	writeActive := func(v bool) { mu.Lock(); *active = v; mu.Unlock() }
	writeTemp := func(v nulltypes.NullFloat64) { mu.Lock(); *temp = v; mu.Unlock() }

	unknownConnectionStatus := int64(2)
	state := ConnectState{
		Type:   5,
		Value1: readTemp(),
		// No heartbeat sample yet.
		Value2: &unknownConnectionStatus,
	}
	heartbeat := time.NewTicker(heartbeatEvery)
	defer heartbeat.Stop()
	connbeat := time.NewTicker(cloudEvery)
	defer connbeat.Stop()

	if sett := readSettings(); cloudEnabled(&sett) {
		cloudWorker.submit(cloudSendRequest{
			writeToken: cloudWriteToken(&sett),
			url:        cloudStateURL(sett.ConDev),
			state:      state,
			initial:    true,
		})
	}

	for {
		select {
		case <-ctx.Done():
			log.Println("Perioder shutting down...")
			return
		case <-connbeat.C:
			sett := readSettings()
			if cloudEnabled(&sett) {
				isActive, temperature := readHealth()
				state = currentCloudState(isActive, temperature)
				cloudWorker.submit(cloudSendRequest{
					writeToken: cloudWriteToken(&sett),
					url:        cloudStateURL(sett.ConDev),
					state:      state,
				})
			}

		case <-heartbeat.C:
			sett := readSettings()
			isActive := true

			if sett.NetEn && len(sett.Net) > 0 {
				log.Printf("Checking network connectivity to %s", sett.Net)
				if err := ping(sett.Net); err != nil {
					log.Printf("Network check failed for %s: %v", sett.Net, err)
					isActive = false
				} else {
					log.Printf("Network check passed for %s", sett.Net)
				}
			}

			if isActive && sett.ProcEn && len(sett.Proc) > 0 {
				if found, err := FindProcess(sett.Proc); !found || err != nil {
					if err != nil {
						log.Printf("Process check error: %v", err)
					} else {
						log.Printf("Process %s not found", sett.Proc)
					}
					isActive = false
				}
			}

			if isActive && sett.Pause {
				log.Printf("Pause mode enabled, setting inactive")
				isActive = false
			}

			if isActive {
				resp, err := sendrecv(ctx, "~U", ch)
				ok := err == nil && resp == "~A"
				writeActive(ok)

				if ok {
					if tbuf, err := sendrecv(ctx, "~G", ch); err == nil {
						writeTemp(parseTemperature(tbuf))
					} else {
						log.Printf("Temperature read failed: %v", err)
						writeTemp(invalidTemperature())
					}
				} else {
					writeTemp(invalidTemperature())
				}
			} else {
				writeActive(false)
				writeTemp(invalidTemperature())
			}
		}
	}
}
