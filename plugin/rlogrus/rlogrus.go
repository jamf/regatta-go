// Copyright JAMF Software, LLC

package rlogrus

import (
	"github.com/sirupsen/logrus"
)

type Logger struct {
	ll *logrus.Logger
}

func (l *Logger) Infof(s string, a ...any) {
	l.ll.Infof(s, a...)
}

func (l *Logger) Debugf(s string, a ...any) {
	l.ll.Debugf(s, a...)
}

func (l *Logger) Warnf(s string, a ...any) {
	l.ll.Warnf(s, a...)
}

func (l *Logger) Errorf(s string, a ...any) {
	l.ll.Errorf(s, a...)
}

// New builds `client.Logger` implementation using provided logrus.Logger.
func New(ll *logrus.Logger) *Logger {
	return &Logger{
		ll: ll,
	}
}
