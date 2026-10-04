package lock

import "errors"

var ErrBusy = errors.New("locked by another process")
