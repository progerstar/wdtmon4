package main

import (
	"bufio"
	"time"

	"go.bug.st/serial"
)

func ReadLine(port serial.Port, timeout time.Duration) string {
	s := make(chan string)

	go func() {
		line, err := bufio.NewReader(port).ReadString('\n')
		if err != nil {
			s <- ""
		} else {
			s <- line
		}
		close(s)
	}()

	select {
	case line := <-s:
		return line
	case <-time.After(timeout):
		return ""
	}
}

func serialWorker(portName string, ch chan string) {
	var res string

	for {
		s := <-ch
		res = ""
		port, err := serial.Open(portName, &serial.Mode{BaudRate: 115200})
		if err != nil {
			ch <- ""
			continue
		}
		port.SetReadTimeout(1 * time.Second)

		_, err = port.Write([]byte(s))
		if err == nil {
			res = ReadLine(port, 1*time.Second)
		}

		port.Close()
		ch <- res
	}

}
