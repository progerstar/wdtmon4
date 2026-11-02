package main

import (
	"fmt"
	"log"
)

// conditionLog reports a condition that is checked over and over (every
// heartbeat, every UI poll) once when it starts or changes and once when it
// ends, instead of on every check.
type conditionLog struct {
	active string
}

// set logs the message unless it is the condition already reported.
func (c *conditionLog) set(format string, args ...any) {
	message := fmt.Sprintf(format, args...)
	if message == c.active {
		return
	}
	c.active = message
	log.Print(message)
}

// clear logs the message only if a condition was reported before.
func (c *conditionLog) clear(format string, args ...any) {
	if c.active == "" {
		return
	}
	c.active = ""
	log.Printf(format, args...)
}
