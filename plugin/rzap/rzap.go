// Copyright JAMF Software, LLC

package rzap

import (
	"go.uber.org/zap"
)

type Logger struct {
	zl *zap.SugaredLogger
}

func (l *Logger) Infof(s string, a ...any) {
	l.zl.Infof(s, a...)
}

func (l *Logger) Debugf(s string, a ...any) {
	l.zl.Debugf(s, a...)
}

func (l *Logger) Warnf(s string, a ...any) {
	l.zl.Warnf(s, a...)
}

func (l *Logger) Errorf(s string, a ...any) {
	l.zl.Errorf(s, a...)
}

// New builds `client.Logger` implementation using provided zap.Logger.
func New(zl *zap.Logger) *Logger {
	return &Logger{
		zl: zl.Sugar(),
	}
}
