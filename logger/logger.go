// -*- Mode: Go; indent-tabs-mode: t -*-

/*
 * Copyright 2023 Canonical Ltd.
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
	"log"
	"os"
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
)

// Init sets up the logging system with screen output.
func Init(lv Level) {
	logLevel = lv
	log.SetFlags(log.Ldate | log.Lmicroseconds)
	log.SetOutput(os.Stdout)
}

// Close finalizes the logging system.
func Close() {
}

// SetLevel updates the current logging level.
func SetLevel(lv Level) {
	logLevel = lv
}

// Info logs informational messages.
func Info(v ...interface{}) {
	if logLevel >= InfoLevel {
		log.Print(InfoPrefix, fmt.Sprint(v...))
	}
}

// Infof logs formatted informational messages.
func Infof(format string, v ...interface{}) {
	if logLevel >= InfoLevel {
		log.Printf(InfoPrefix+"%s", fmt.Sprintf(format, v...))
	}
}

// Warning logs messages requiring user attention.
func Warning(v ...interface{}) {
	if logLevel >= WarningLevel {
		log.Print(WarningPrefix, fmt.Sprint(v...))
	}
}

// Warningf logs formatted messages requiring user attention.
func Warningf(format string, v ...interface{}) {
	if logLevel >= WarningLevel {
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
	if logLevel >= DebugLevel {
		log.Print(DebugPrefix, fmt.Sprint(v...))
	}
}

// Debugf logs formatted messages to help developers follow code execution.
func Debugf(format string, v ...interface{}) {
	if logLevel >= DebugLevel {
		log.Printf(DebugPrefix+"%s", fmt.Sprintf(format, v...))
	}
}
