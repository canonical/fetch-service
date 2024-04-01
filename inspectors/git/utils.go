package git

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"time"

	"github.com/canonical/fetch-service/logger"
)

// UnpackObjects creates a work tree in dir from a file containing
// packed objects.
func UnpackObjects(f io.ReadSeeker, dir string) error {
	logger.Info("unpacking git objects")

	cmd := execCommand("git", "init-db", "-q")
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		return err
	}

	// FIXME: parse protocol for want-refs
	_, err := decodeGitProtocol(f)
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

	cmd = execCommand(path, "-q")
	cmd.Dir = dir
	pipe, err := cmd.StdinPipe()
	if err != nil {
		return err
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("git-unpack-objects execution error: %s", err)
	}

	err = readPack(f, pipe)
	if err != nil {
		pipe.Close()
		cmd.Wait()
		return fmt.Errorf("error reading git pack: %s", err)
	}

	// this shouldn't be necessary but strange things happen if you're
	// running on a RAM-backed filesystem
	time.Sleep(200 * time.Millisecond)

	pipe.Close()

	return cmd.Wait()
}

func Checkout(dir, ref string) error {
	logger.Debugf("checkout git ref %s", ref)

	cmd := execCommand("git", "checkout", "-q", ref)
	cmd.Dir = dir
	return cmd.Run()
}

func readPack(f io.Reader, w io.Writer) error {
	buf := make([]byte, 4)
	var err error

	for {
		_, err = f.Read(buf)
		if err != nil {
			return err
		}

		var size int64
		size, err = strconv.ParseInt(string(buf), 16, 32)
		if err != nil {
			return err
		}

		if size == 0 {
			break
		}

		x := make([]byte, 1)
		if _, err = f.Read(x); err != nil {
			return err
		}

		databuf := make([]byte, size-5)
		if _, err = f.Read(databuf); err != nil {
			return err
		}

		if _, err = w.Write(databuf); err != nil {
			return err
		}
	}

	return nil
}

func execCommand(name string, args ...string) *exec.Cmd {
	logger.Debugf("command to execute: %s %v", name, args)
	cmd := exec.Command(name, args...)
	return cmd
}

func sliceIndex(s1 []byte, s2 []byte) int {
	l1 := len(s1)
	l2 := len(s2)

	if l1 < l2 {
		return -1
	}

	for n := 0; n < l1-l2; n++ {
		if bytes.Equal(s1[n:n+l2], s2) {
			return n
		}
	}

	return -1
}
