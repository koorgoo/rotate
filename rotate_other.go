// +build !linux

package rotate

// Dirname returns directory name containing fd.
func Dirname(fd uintptr) (string, error) {
	return "", ErrNotSupported
}
