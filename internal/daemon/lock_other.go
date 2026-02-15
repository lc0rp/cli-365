//go:build !unix

package daemon

import "os"

func acquireFileLock(_ string) (*os.File, error) {
	return nil, ErrUnsupportedPlatform
}

func releaseFileLock(_ string, _ *os.File) error {
	return nil
}
