package lock

import (
	"errors"
	"time"
)

var ErrBusy = errors.New("locked by another process")

func TryFor(path string, wait time.Duration) (func(), error) {
	deadline := time.Now().Add(wait)
	for {
		release, err := Try(path)
		if !errors.Is(err, ErrBusy) || !time.Now().Before(deadline) {
			return release, err
		}
		time.Sleep(200 * time.Millisecond)
	}
}
