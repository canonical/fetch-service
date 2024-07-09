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

package files

import (
	"os"
)

// ArtefactFile is an implementation of ArtefactReader.
type ArtefactFile struct {
	f    *os.File
	size int64
}

// NewArtefactFile creates an ArtefactFile from os.File.
func NewArtefactFile(f *os.File) (*ArtefactFile, error) {
	st, err := f.Stat()
	if err != nil {
		return nil, err
	}
	return &ArtefactFile{
		f:    f,
		size: st.Size(),
	}, nil
}

// OpenArtefactFile opens a downloaded artefact file for reading.
func OpenArtefactFile(filename string) (*ArtefactFile, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	return NewArtefactFile(f)
}

// Read reads up to len(b) bytes from the ArtefactFile and stores them
// in b. It returns the numeber of read bytes and any error encountered.
func (f *ArtefactFile) Read(b []byte) (int, error) {
	return f.f.Read(b)
}

// ReadAt reads len(b) bytes from the ArtefactFile starting at byte
// offset off. It returns the number of bytes read and the error, if any.
func (f *ArtefactFile) ReadAt(b []byte, off int64) (int, error) {
	return f.f.ReadAt(b, off)
}

// Seek sets the offset for the next Read or Write on file to offset,
// interpreted according to whence. It returns the new offset and
// an error, if any.
func (f *ArtefactFile) Seek(off int64, whence int) (int64, error) {
	return f.f.Seek(off, whence)
}

// Len returns the size of the ArtefactFile.
func (f *ArtefactFile) Len() int {
	return int(f.size)
}

// Close closes the ArtefactFile, rendering it unusable for I/O.
func (f *ArtefactFile) Close() error {
	return f.f.Close()
}
