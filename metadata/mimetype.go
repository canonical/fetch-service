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

package metadata

import (
	"bytes"
	"encoding/binary"
	"regexp"

	"github.com/gabriel-vasile/mimetype"
)

func init() {
	mimetype.SetLimit(1 << 30) // input data is mmapped
	mimetype.Lookup("application/zip").Extend(whlDetector, "application/x-python-wheel", ".whl")
}

// The zip tokenizer is based on the implementation in
// github.com/gabriel-vasile/mimetype.

// zipTokenizer holds the source zip file and scanned index.
type zipTokenizer struct {
	in []byte
	i  int // current index
}

func (t *zipTokenizer) next() (fileName string) {
	if t.i > len(t.in) {
		return
	}
	in := t.in[t.i:]
	// pkSig is the signature of the zip local file header.
	pkSig := []byte("PK\003\004")
	pkIndex := bytes.Index(in, pkSig)
	// 30 is the offset of the file name in the header.
	fNameOffset := pkIndex + 30
	// end if signature not found or file name offset outside of file.
	if pkIndex == -1 || fNameOffset > len(in) {
		return
	}

	fNameLen := int(binary.LittleEndian.Uint16(in[pkIndex+26 : pkIndex+28]))
	if fNameLen <= 0 || fNameOffset+fNameLen > len(in) {
		return
	}
	t.i += fNameOffset + fNameLen
	return string(in[fNameOffset : fNameOffset+fNameLen])
}

// zipMatches returns true if the zip file headers from in matches any of the path patterns.
func zipMatches(in []byte, patterns ...string) bool {
	t := zipTokenizer{in: in}
	for i, tok := 0, t.next(); tok != ""; i, tok = i+1, t.next() {
		for _, p := range patterns {
			if matched, _ := regexp.MatchString(p, tok); matched {
				return true
			}
		}
	}

	return false
}
