// -*- Mode: Go; indent-tabs-mode: t -*-

/*
 * Copyright 2023-2025 Canonical Ltd.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License version 3 as
 * published by the Free Software Foundation.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 *
 */

package logger

import (
	"fmt"
	"io"
	"log"
	"os"
	"sync"
)

const (
	ErrorLevel Level = iota
	WarningLevel
	InfoLevel
	DebugLevel
)

const (
	ErrorPrefix   = "ERROR: "
	WarningPrefix = "WARN : "
	InfoPrefix    = "INFO : "
	DebugPrefix   = "DEBUG: "
)

type Level int

var (
	logLevel Level
	logFile  *os.File
	logLock  sync.RWMutex
)

// Init sets up the logging system.
// If "logFilepath" is empty, logging is done to standard out.
func Init(lv Level, logFilepath string) error {
	SetLevel(lv)
	log.SetFlags(log.Ldate | log.Lmicroseconds)
	if logFilepath != "" {
		return SetLogFile(logFilepath)
	}
	log.SetOutput(os.Stdout)
	return nil
}

// Close finalizes the logging system.
func Close() {
	if logFile != nil {
		if err := logFile.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "cannot close logfile: %s\n", err)
		}
	} else {
		log.SetOutput(io.Discard)
	}
}

// SetLevel updates the current logging level.
func SetLevel(lv Level) {
	logLock.Lock()
	defer logLock.Unlock()

	logLevel = lv
}

func SetLogFile(logPath string) error {
	var err error
	logFile, err = os.OpenFile(logPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("set log file: %w", err)
	}
	log.SetOutput(logFile)
	return nil
}

type Logger interface {
	Info(v ...interface{})
	Infof(format string, v ...interface{})
	Warning(v ...interface{})
	Warningf(format string, v ...interface{})
	Error(v ...interface{})
	Errorf(format string, v ...interface{})
	Debug(v ...interface{})
	Debugf(format string, v ...interface{})
}

type SessionLogger struct {
	prefix string
}

func NewSessionLogger(sid string) SessionLogger {
	return SessionLogger{prefix: sessionFormat(sid)}
}

func sessionFormat(sid string) string {
	return fmt.Sprintf("[%s] ", sid)
}

func (slog SessionLogger) Info(v ...interface{}) {
	args := []interface{}{slog.prefix}
	Info(append(args, v...)...)
}

func (slog SessionLogger) Infof(format string, v ...interface{}) {
	Infof(slog.prefix+format, v...)
}

func (slog SessionLogger) Warning(v ...interface{}) {
	args := []interface{}{slog.prefix}
	Warning(append(args, v...)...)
}

func (slog SessionLogger) Warningf(format string, v ...interface{}) {
	Warningf(slog.prefix+format, v...)
}

func (slog SessionLogger) Error(v ...interface{}) {
	args := []interface{}{slog.prefix}
	Error(append(args, v...)...)
}

func (slog SessionLogger) Errorf(format string, v ...interface{}) {
	Errorf(slog.prefix+format, v...)
}

func (slog SessionLogger) Debug(v ...interface{}) {
	args := []interface{}{slog.prefix}
	Debug(append(args, v...)...)
}

func (slog SessionLogger) Debugf(format string, v ...interface{}) {
	Debugf(slog.prefix+format, v...)
}

// Info logs informational messages.
func Info(v ...interface{}) {
	if atLeastLogLevel(InfoLevel) {
		log.Print(InfoPrefix, fmt.Sprint(v...))
	}
}

// Infof logs formatted informational messages.
func Infof(format string, v ...interface{}) {
	if atLeastLogLevel(InfoLevel) {
		log.Printf(InfoPrefix+"%s", fmt.Sprintf(format, v...))
	}
}

// Warning logs messages requiring user attention.
func Warning(v ...interface{}) {
	if atLeastLogLevel(WarningLevel) {
		log.Print(WarningPrefix, fmt.Sprint(v...))
	}
}

// Warningf logs formatted messages requiring user attention.
func Warningf(format string, v ...interface{}) {
	if atLeastLogLevel(WarningLevel) {
		log.Printf(WarningPrefix+"%s", fmt.Sprintf(format, v...))
	}
}

// Error logs messages reporting incorrect behavior.
func Error(v ...interface{}) {
	log.Print(ErrorPrefix, fmt.Sprint(v...))
}

// Errorf logs formatted messages reporting incorrect behavior.
func Errorf(format string, v ...interface{}) {
	log.Printf(ErrorPrefix+"%s", fmt.Sprintf(format, v...))
}

// Fatal logs messages reporting incorrect behavior.
func Fatal(v ...interface{}) {
	log.Fatal(ErrorPrefix, fmt.Sprint(v...))
}

// Fatalf logs formatted messages reporting incorrect behavior.
func Fatalf(format string, v ...interface{}) {
	log.Fatalf(ErrorPrefix+"%s", fmt.Sprintf(format, v...))
}

// Debug logs messages to help developers follow code execution.
func Debug(v ...interface{}) {
	if atLeastLogLevel(DebugLevel) {
		log.Print(DebugPrefix, fmt.Sprint(v...))
	}
}

// Debugf logs formatted messages to help developers follow code execution.
func Debugf(format string, v ...interface{}) {
	if atLeastLogLevel(DebugLevel) {
		log.Printf(DebugPrefix+"%s", fmt.Sprintf(format, v...))
	}
}

func atLeastLogLevel(lv Level) bool {
	logLock.RLock()
	defer logLock.RUnlock()
	return logLevel >= lv
}
