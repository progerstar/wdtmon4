package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/keybase/go-ps"
)

func ping(host string) error {
	seconds := 5
	timeOut := time.Duration(seconds) * time.Second
	conn, err := net.DialTimeout("tcp", host+":80", timeOut)
	if err == nil {
		conn.Close()
	}
	//fmt.Printf("Remote Address : %s \n", conn.RemoteAddr().String())
	//fmt.Printf("Local Address : %s \n", conn.LocalAddr().String())
	return err
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
}

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

func sendrecv(cmd string, ch chan string) string {
	ch <- cmd
	res := <-ch
	i := strings.Index(res, recvchecker(cmd[:2]))
	if i > -1 {
		return res[i:strings.Index(res[i:], "\n")]
	} else {
		return "Err"
	}
}

func consend(uid string, url string, state *ConnectState) error {
	client := &http.Client{}
	stateB, err := json.Marshal(state)
	if err == nil {
		req, _ := http.NewRequest("POST", url, bytes.NewBuffer(stateB))
		req.Header.Add("Content-Type", "application/json")
		req.Header.Add("Content-Length", strconv.Itoa(len(stateB)))
		req.Header.Add("id", uid)
		resp, err := client.Do(req)
		if err == nil {
			resp.Body.Close()
		}
	}
	return err
}

func perioder(cloudEn bool, settings *Settings, ch chan string, active *bool, temp *NullFloat64) {
	heartbeat := time.Tick(2 * time.Second)
	connbeat := time.Tick(5 * time.Minute)
	state := ConnectState{Type: 5, Value1: *temp, Value2: 2, Alias: settings.ConAlias}

	if cloudEn {
		if consend(settings.ConUID, CLOUD_URL+"/state/"+settings.ConDev, &state) == nil {
			state.Value2 = 1
		}
	}

	for {
		time.Sleep(1 * time.Second)
		select {
		case <-connbeat:
			if settings.ConEn && cloudEn && *active && len(settings.ConUID) > 0 {
				state.Value1 = *temp
				if consend(settings.ConUID, CLOUD_URL+"/state/"+settings.ConDev, &state) == nil {
					state.Value2 = 1
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
				*active = (sendrecv("~U", ch) == "~A")
				tbuf := sendrecv("~G", ch)

				if len(tbuf) == 6 {
					if n, err := strconv.ParseFloat(tbuf[2:6], 64); err == nil {
						*temp = NullFloat64{n / 10, true}
					} else {
						*temp = NullFloat64{0, false}
					}
				} else {
					*temp = NullFloat64{0, false}
				}
			}
		}
	}
}
