//go:build unix

package daemon

import (
	"errors"
	"os"
	"strconv"
	"syscall"
)

func acquireFileLock(path string) (*os.File, error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	if err := os.Chmod(path, 0o600); err != nil {
		_ = file.Close()
		return nil, err
	}

	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = file.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) || errors.Is(err, syscall.EAGAIN) {
			return nil, ErrAlreadyRunning
		}
		return nil, err
	}

	if err := file.Truncate(0); err == nil {
		_, _ = file.Seek(0, 0)
		_, _ = file.WriteString(strconv.Itoa(os.Getpid()))
	}

	return file, nil
}

func releaseFileLock(path string, file *os.File) error {
	if file == nil {
		return nil
	}
	_ = syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
	_ = file.Close()
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
