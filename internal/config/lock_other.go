//go:build !unix

package config

// No scheduler on these platforms: runs are manual and never overlap.
func AcquireLock() (func(), error) {
	return func() {}, nil
}
