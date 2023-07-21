// Copyright JAMF Software, LLC

package client

import (
	"log"
)

type Logger interface {
	Info(...any)
	Infof(string, ...any)
	Debug(...any)
	Debugf(string, ...any)
	Warn(...any)
	Warnf(string, ...any)
	Fatal(...any)
	Fatalf(string, ...any)
	Fatalln(...any)
	Print(...any)
	Printf(string, ...any)
	Println(...any)
	Error(...any)
	Errorf(string, ...any)
}

var defaultLogger Logger = &PrintLogger{}

type PrintLogger struct{}

func (p PrintLogger) Info(a ...any) {
	log.Print(a...)
}

func (p PrintLogger) Infof(s string, a ...any) {
	log.Printf(s, a...)
}

func (p PrintLogger) Debug(a ...any) {
	log.Print(a...)
}

func (p PrintLogger) Debugf(s string, a ...any) {
	log.Printf(s, a...)
}

func (p PrintLogger) Warn(a ...any) {
	log.Print(a...)
}

func (p PrintLogger) Warnf(s string, a ...any) {
	log.Printf(s, a...)
}

func (p PrintLogger) Error(a ...any) {
	log.Print(a...)
}

func (p PrintLogger) Errorf(s string, a ...any) {
	log.Printf(s, a...)
}

func (p PrintLogger) Fatal(a ...any) {
	log.Print(a...)
}

func (p PrintLogger) Fatalf(s string, a ...any) {
	log.Printf(s, a...)
}

func (p PrintLogger) Fatalln(a ...any) {
	log.Print(a...)
}

func (p PrintLogger) Print(a ...any) {
	log.Print(a...)
}

func (p PrintLogger) Printf(s string, a ...any) {
	log.Printf(s, a...)
}

func (p PrintLogger) Println(a ...any) {
	log.Print(a...)
}
