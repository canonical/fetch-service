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
 */
package git

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"time"

	"github.com/canonical/fetch-service/logger"
)

type GitPackfileSideband int8

const (
	PackfileData GitPackfileSideband = iota + 1
	PackfileProgress
	PackfileErrors
)

// UnpackObjects creates a work tree in dir from a file containing
// packed objects.
func UnpackObjects(f io.ReadSeeker, dir string, slog logger.Logger) error {
	slog.Info("unpacking git objects")

	cmd := execCommand(slog, "git", "init-db", "-q")
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		return err
	}

	// FIXME: parse protocol for want-refs
	_, err := decodeGitProtocol(f, slog)
	if err != nil {
		return err
	}

	path := "/usr/lib/git-core/git-unpack-objects"
	if _, err := os.Stat(path); err != nil {
		path = "/snap/fetch-service/current/usr/lib/git-core/git-unpack-objects"
		if _, err := os.Stat(path); err != nil {
			return err
		}
	}

	cmd = execCommand(slog, path, "-q")
	cmd.Dir = dir
	pipe, err := cmd.StdinPipe()
	if err != nil {
		return err
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("git-unpack-objects execution error: %s", err)
	}

	err = readPack(f, pipe, slog)
	if err != nil {
		if err := pipe.Close(); err != nil {
			return err
		}
		if err := cmd.Wait(); err != nil {
			slog.Errorf("error waiting for git-unpack-objects: %s", err)
		}
		return fmt.Errorf("error reading git pack: %s", err)
	}

	// this shouldn't be necessary but strange things happen if you're
	// running on a RAM-backed filesystem
	time.Sleep(200 * time.Millisecond)

	pipe.Close()

	return cmd.Wait()
}

func Checkout(dir, ref string, slog logger.Logger) error {
	slog.Debugf("checkout git ref %s", ref)

	cmd := execCommand(slog, "git", "checkout", "-q", ref)
	cmd.Dir = dir
	return cmd.Run()
}

func readPack(f io.Reader, w io.Writer, slog logger.Logger) error {
	slog.Debug("read git pack")
	buf := make([]byte, 4)
	var err error

	for {
		_, err = f.Read(buf)
		if err != nil {
			return err
		}

		// From the git protocol documentation:
		// "Each packet starting with the packet-line length of the amount of
		// data that follows, followed by a single byte specifying the sideband
		// the following data is coming in on."

		var size int64
		size, err = strconv.ParseInt(string(buf), 16, 32)
		if err != nil {
			return err
		}

		// 0005x means 5 bytes total, 4 bytes containing the size (5) and one
		// byte is the sideband.

		if size < 5 {
			slog.Debugf("pack size %04x", size)
			break
		}

		git_sideband_byte := make([]byte, 1)

		if _, err := f.Read(git_sideband_byte); err != nil {
			return err
		}

		databuf := make([]byte, size-5)
		if _, err = f.Read(databuf); err != nil {
			return err
		}

		// From the git protocol documentation:
		// "The sideband byte will be a 1, 2 or a 3. Sideband 1 will contain packfile
		// data, sideband 2 will be used for progress information that the client will
		// generally print to stderr and sideband 3 is used for error information."

		if GitPackfileSideband(git_sideband_byte[0]) != PackfileData {
			slog.Debugf("Non-package data, skipping...")
			continue
		}

		// Only sideband 1 is processed by the git inspector.

		if _, err = w.Write(databuf); err != nil {
			return err
		}
	}

	return nil
}

func execCommand(slog logger.Logger, name string, args ...string) *exec.Cmd {
	slog.Debugf("command to execute: %s %v", name, args)
	cmd := exec.Command(name, args...)
	return cmd
}
