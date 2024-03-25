// -*- Mode: Go; indent-tabs-mode: t -*-

/*
 * Copyright 2024 Canonical Ltd.
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
	"slices"
	"strconv"

	"github.com/canonical/fetch-service/logger"
	"github.com/canonical/fetch-service/metadata"
)

func getGitProtocol(a *metadata.Artefact) string {
	proto, ok := a.CurrentDownload.RequestHeader["Git-Protocol"]
	if !ok || len(proto) != 1 {
		return ""
	}
	return proto[0]
}

func decodeGitProtocol(buf []byte) ([]string, error) {
	msgs := []string{}
	for {
		if len(buf) < 4 {
			return msgs, errors.New("git protocol decode error")
		}

		size, err := strconv.ParseUint(string(buf[:4]), 16, 32)
		if err != nil {
			return msgs, err
		}

		switch size {
		case 0, 2: // flush or end
			return msgs, nil
		case 1: // delim
			size = 4
		case 3, 4: // error
			return msgs, errors.New("git protocol decode error")
		}

		if size > uint64(len(buf)) {
			return msgs, errors.New("git protocol short message error")
		}

		// stop decoding if packfile found
		if size == 13 && slices.Equal(buf[4:13], []byte("packfile\n")) {
			return msgs, nil
		}

		m := string(buf[4:size])
		msgs = append(msgs, string(m))
		logger.Debugf(":: %s", m)

		buf = buf[size:]
	}
}
