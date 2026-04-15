// -*- Mode: Go; indent-tabs-mode: t -*-

/*
 * Copyright 2024-2025 Canonical Ltd.
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

package git

import (
	"errors"
	"fmt"
	"io"
	"strconv"

	. "github.com/canonical/fetch-service/inspectors/common"
	"github.com/canonical/fetch-service/logger"
)

func getGitProtocol(a RequestArtifact) string {
	proto, ok := a.RequestHeader("Git-Protocol")
	if !ok || len(proto) != 1 {
		return ""
	}
	return proto[0]
}

func decodeGitProtocol(f io.Reader, sl logger.Logger) ([]string, error) {
	msgs := []string{}
	for {
		buf := make([]byte, 4)
		var err error
		if _, err = f.Read(buf); err != nil {
			return msgs, fmt.Errorf("decode error")
		}

		size, err := strconv.ParseUint(string(buf), 16, 32)
		if err != nil {
			return msgs, err
		}

		switch size {
		case 0, 2: // flush or end
			return msgs, nil
		case 1: // delim
			size = 4
		case 3, 4: // error
			return msgs, errors.New("decode error")
		}

		line := make([]byte, size-4)
		if _, err = f.Read(line); err != nil {
			return msgs, fmt.Errorf("cannot read %d bytes from input: %w", size, err)
		}

		if len(line) < 256 {
			sl.Debugf(":: %04x  %q", size, line)
		} else {
			sl.Debugf(":: %04x  <line too long>", size)
		}

		// remove trailing line break, if any
		l := len(line)
		if l > 0 && line[l-1] == '\n' {
			line = line[:l-1]
		}

		l = len(line)
		// remove carriage return (HTTP/1.1)
		if l > 0 && line[l-1] == '\r' {
			line = line[:l-1]
		}

		msgs = append(msgs, string(line))

		// stop decoding if packfile found
		if string(line) == "packfile" || string(line) == "\x01packfile" {
			return msgs, nil
		}

		// stop decoding if NAK found
		if string(line) == "NAK" {
			return msgs, nil
		}
	}
}
