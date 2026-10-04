//go:build !unix

package lock

// No scheduler on these platforms: runs are manual and never overlap.
func Try(string) (func(), error) {
	return func() {}, nil
}
