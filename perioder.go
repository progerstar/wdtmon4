package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/datumbrain/nulltypes"
	"github.com/keybase/go-ps"
)

const (
	defaultTimeout    = 5 * time.Second
	heartbeatInterval = 2 * time.Second
	connbeatInterval  = 5 * time.Minute
)

func ping(hostOrURL string) error {
	var host string

	if strings.Contains(hostOrURL, "://") {
		parsedURL, err := url.Parse(hostOrURL)
		if err != nil {
			return fmt.Errorf("invalid URL format: %v", err)
		}
		host = parsedURL.Hostname()
		if host == "" {
			return fmt.Errorf("cannot extract hostname from URL: %s", hostOrURL)
		}
	} else {
		host = hostOrURL
	}

	//log.Printf("Pinging host: %s (from input: %s)", host, hostOrURL)

	conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, "80"), defaultTimeout)
	if err != nil {
		return err
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

func sendrecv(cmd string, ch chan string) (string, error) {
	select {
	case ch <- cmd:
		res := <-ch

		if res == "" {
			return "", errors.New("empty response")
		}
		return strings.TrimSpace(res), nil

	case <-time.After(2 * time.Second):
		return "", errors.New("channel timeout")
	}
}

func consend(ctx context.Context, uid, url string, state *ConnectState, client *http.Client) error {
	if client == nil {
		client = &http.Client{Timeout: defaultTimeout}
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
	req.Header.Set("Content-Length", strconv.Itoa(len(stateBytes)))
	req.Header.Set("id", uid)

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return errors.New("server returned error status: " + resp.Status)
	}

	return nil
}

func parseTemperature(tbuf string) nulltypes.NullFloat64 {
	if strings.Contains(tbuf, "EEEE") {
		return nulltypes.NullFloat64{Float64: 0, Valid: false}
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
	return nulltypes.NullFloat64{Float64: 0, Valid: false}
}

func perioder(ctx context.Context, cloudEn bool, settings *Settings, ch chan string, active *bool, temp *nulltypes.NullFloat64) {
	log.Println("Perioder started")
	defer log.Println("Perioder stopped")

	client := &http.Client{Timeout: defaultTimeout}

	state := ConnectState{
		Type:   5,
		Value1: *temp,
		Value2: 2,
		Alias:  settings.ConAlias,
	}

	heartbeat := time.NewTicker(heartbeatInterval)
	defer heartbeat.Stop()
	connbeat := time.NewTicker(connbeatInterval)
	defer connbeat.Stop()

	if cloudEn && len(settings.ConUID) > 0 {
		if err := consend(ctx, settings.ConUID, CLOUD_URL+"/state/"+settings.ConDev, &state, client); err == nil {
			state.Value2 = 1
		} else {
			log.Printf("Initial cloud send failed: %v", err)
		}
	}

	for {
		select {
		case <-ctx.Done():
			log.Println("Perioder shutting down...")
			return
		case <-connbeat.C:
			if settings.ConEn && cloudEn && *active && len(settings.ConUID) > 0 {
				state.Value1 = *temp
				if err := consend(ctx, settings.ConUID, CLOUD_URL+"/state/"+settings.ConDev, &state, client); err != nil {
					log.Printf("Cloud send failed: %v", err)
					state.Value2 = 0
				} else {
					state.Value2 = 1
				}
			}

		case <-heartbeat.C:
			isActive := true

			if settings.NetEn && len(settings.Net) > 0 {
				log.Printf("Checking network connectivity to %s", settings.Net)
				if err := ping(settings.Net); err != nil {
					log.Printf("Network check failed for %s: %v", settings.Net, err)
					isActive = false
				} else {
					log.Printf("Network check passed for %s", settings.Net)
				}
			}

			if isActive && settings.ProcEn && len(settings.Proc) > 0 {
				if found, err := FindProcess(settings.Proc); !found || err != nil {
					if err != nil {
						log.Printf("Process check error: %v", err)
					} else {
						log.Printf("Process %s not found", settings.Proc)
					}
					isActive = false
				}
			}

			if isActive && settings.Pause {
				log.Printf("Pause mode enabled, setting inactive")
				isActive = false
			}

			if isActive {
				resp, err := sendrecv("~U", ch)
				*active = err == nil && resp == "~A"

				if *active {
					if tbuf, err := sendrecv("~G", ch); err == nil {
						*temp = parseTemperature(tbuf)
					} else {
						log.Printf("Temperature read failed: %v", err)
						*temp = nulltypes.NullFloat64{Float64: 0, Valid: false}
					}
				}
			} else {
				*active = false
				*temp = nulltypes.NullFloat64{Float64: 0, Valid: false}
			}
		}
	}
}
