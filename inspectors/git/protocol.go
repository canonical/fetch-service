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

func decodeGitProtocol(buf []byte) (msgs []string, err error) {
	for {
		if len(buf) < 4 {
			err = errors.New("git protocol decode error")
			return
		}

		// consistency check
		if buf[0] != 0x30 {
			err = errors.New("git protocol long message error")
			return
		}

		var size uint64
		size, err = strconv.ParseUint(string(buf[:4]), 16, 32)
		if err != nil {
			return
		}

		switch size {
		case 0, 3, 4: // flush or end
			return
		case 1: // delim
			size = 4
		}

		if size > uint64(len(buf)) {
			err = errors.New("git protocol short message error")
			return
		}

		// stop decoding if packfile found
		if size == 13 && slices.Equal(buf[4:13], []byte("packfile\n")) {
			return
		}

		m := string(buf[4:size])
		msgs = append(msgs, string(m))
		logger.Debugf(":: %s", m)

		buf = buf[size:]
	}
}
