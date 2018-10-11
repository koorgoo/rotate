// +build !linux

package rotate

// Dirname returns a directory containing fd.
func Dirname(fd uintptr) (string, error) {
	return "", ErrNotSupported
}
