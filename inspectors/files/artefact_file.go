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

// ArtifactFile is an implementation of ArtifactReader.
type ArtifactFile struct {
	f    *os.File
	size int64
}

// NewArtifactFile creates an ArtifactFile from os.File.
func NewArtifactFile(f *os.File) (*ArtifactFile, error) {
	st, err := f.Stat()
	if err != nil {
		return nil, err
	}
	return &ArtifactFile{
		f:    f,
		size: st.Size(),
	}, nil
}

// OpenArtifactFile opens a downloaded artifact file for reading.
func OpenArtifactFile(filename string) (*ArtifactFile, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	return NewArtifactFile(f)
}

// Read reads up to len(b) bytes from the ArtifactFile and stores them
// in b. It returns the numeber of read bytes and any error encountered.
func (f *ArtifactFile) Read(b []byte) (int, error) {
	return f.f.Read(b)
}

// ReadAt reads len(b) bytes from the ArtifactFile starting at byte
// offset off. It returns the number of bytes read and the error, if any.
func (f *ArtifactFile) ReadAt(b []byte, off int64) (int, error) {
	return f.f.ReadAt(b, off)
}

// Seek sets the offset for the next Read or Write on file to offset,
// interpreted according to whence. It returns the new offset and
// an error, if any.
func (f *ArtifactFile) Seek(off int64, whence int) (int64, error) {
	return f.f.Seek(off, whence)
}

// Len returns the size of the ArtifactFile.
func (f *ArtifactFile) Len() int {
	return int(f.size)
}

// Close closes the ArtifactFile, rendering it unusable for I/O.
func (f *ArtifactFile) Close() error {
	return f.f.Close()
}
