package blockchain

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

type ColorLogger struct {
	logger *log.Logger
}

func NewColorLogger() *ColorLogger {
	return &ColorLogger{
		logger: log.New(os.Stdout, "", log.LstdFlags),
	}
}

func (c *ColorLogger) Info(v ...interface{}) {
	c.logger.SetPrefix(Blue + "INFO: " + Reset)
	c.logger.Println(v...)
}

func (c *ColorLogger) Infof(format string, v ...interface{}) {
	c.logger.SetPrefix(Blue + "INFO: " + Reset)
	c.logger.Printf(format, v...)
}

func (c *ColorLogger) Warn(v ...interface{}) {
	c.logger.SetPrefix(Yellow + "WARN: " + Reset)
	c.logger.Println(v...)
}

func (c *ColorLogger) Warnf(format string, v ...interface{}) {
	c.logger.SetPrefix(Yellow + "WARN: " + Reset)
	c.logger.Printf(format, v...)
}

func (c *ColorLogger) Error(v ...interface{}) {
	c.logger.SetPrefix(Red + "ERROR: " + Reset)
	c.logger.Println(v...)
}

func (c *ColorLogger) Errorf(format string, v ...interface{}) {
	c.logger.SetPrefix(Red + "ERROR: " + Reset)
	c.logger.Printf(format, v...)
}
func (c *ColorLogger) Success(v ...interface{}) {
	c.logger.SetPrefix(Green + "SUCCESS: " + Reset)
	c.logger.Println(v...)
}

func (c *ColorLogger) Successf(format string, v ...interface{}) {
	c.logger.SetPrefix(Green + "SUCCESS: " + Reset)
	c.logger.Printf(format, v...)
}

var logger = NewColorLogger()
