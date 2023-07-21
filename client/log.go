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

var defaultLogger Logger = &printLogger{}

type printLogger struct {
}

func (p printLogger) Info(a ...any) {
	log.Print(a)
}

func (p printLogger) Infof(s string, a ...any) {
	log.Printf(s, a...)
}

func (p printLogger) Debug(a ...any) {
	log.Print(a...)
}

func (p printLogger) Debugf(s string, a ...any) {
	log.Printf(s, a...)
}

func (p printLogger) Warn(a ...any) {
	log.Print(a...)
}

func (p printLogger) Warnf(s string, a ...any) {
	log.Printf(s, a...)
}

func (p printLogger) Error(a ...any) {
	log.Print(a...)
}

func (p printLogger) Errorf(s string, a ...any) {
	log.Printf(s, a...)
}

func (p printLogger) Fatal(a ...any) {
	log.Print(a...)
}

func (p printLogger) Fatalf(s string, a ...any) {
	log.Printf(s, a...)
}

func (p printLogger) Fatalln(a ...any) {
	log.Print(a...)
}

func (p printLogger) Print(a ...any) {
	log.Print(a...)
}

func (p printLogger) Printf(s string, a ...any) {
	log.Printf(s, a...)
}

func (p printLogger) Println(a ...any) {
	log.Print(a...)
}
