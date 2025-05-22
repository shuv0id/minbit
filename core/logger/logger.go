package logger

import (
	"log"
	"os"
)

const (
	Reset  = "\033[0m"
	Red    = "\033[31m"
	Green  = "\033[32m"
	Yellow = "\033[33m"
	Blue   = "\033[34m"
)

var logger = NewLogger()

type Logger struct {
	logger *log.Logger
}

func NewLogger() *Logger {
	return &Logger{
		logger: log.New(os.Stdout, "", log.LstdFlags),
	}
}

func (l *Logger) Info(v ...interface{}) {
	l.logger.SetPrefix(Blue + "[INFO] " + Reset)
	l.logger.Println(v...)
}

func (l *Logger) Infof(format string, v ...interface{}) {
	l.logger.SetPrefix(Blue + "[INFO] " + Reset)
	l.logger.Printf(format, v...)
}

func (l *Logger) Warn(v ...interface{}) {
	l.logger.SetPrefix(Yellow + "[WARN] " + Reset)
	l.logger.Println(v...)
}

func (l *Logger) Warnf(format string, v ...interface{}) {
	l.logger.SetPrefix(Yellow + "[WARN] " + Reset)
	l.logger.Printf(format, v...)
}

func (l *Logger) Error(v ...interface{}) {
	l.logger.SetPrefix(Red + "[ERROR] " + Reset)
	l.logger.Println(v...)
}

func (l *Logger) Errorf(format string, v ...interface{}) {
	l.logger.SetPrefix(Red + "[ERROR] " + Reset)
	l.logger.Printf(format, v...)
}
