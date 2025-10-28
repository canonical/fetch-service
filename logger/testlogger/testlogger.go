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

package testlogger

import (
	"bytes"
	"log"

	"github.com/canonical/fetch-service/logger"
)

var logBuffer bytes.Buffer

// Init sets up the logging system for testing.
func Init(lv logger.Level) {
	logBuffer.Reset()
	logger.SetLevel(lv)
	log.SetFlags(log.Ltime)
	log.SetOutput(&logBuffer)
}

// Contains returns whether the message exists in the log buffer.
func Contains(msg string) bool {
	return bytes.Contains(logBuffer.Bytes(), []byte(msg))
}
