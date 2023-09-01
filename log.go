// Copyright JAMF Software, LLC

package client

import (
	"log"
)

type Logger interface {
	Infof(string, ...any)
	Debugf(string, ...any)
	Warnf(string, ...any)
	Errorf(string, ...any)
}

var defaultLogger Logger = &PrintLogger{}

type PrintLogger struct{}

func (p PrintLogger) Infof(s string, a ...any) {
	log.Printf(s, a...)
}

func (p PrintLogger) Debugf(s string, a ...any) {
	log.Printf(s, a...)
}

func (p PrintLogger) Warnf(s string, a ...any) {
	log.Printf(s, a...)
}

func (p PrintLogger) Errorf(s string, a ...any) {
	log.Printf(s, a...)
}
