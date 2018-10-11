// +build linux

package rotate

import (
	"fmt"
	"os"
	"path"
)

// Dirname returns a directory containing fd.
func Dirname(fd uintptr) (string, error) {
	proc := fmt.Sprintf("/proc/self/fd/%d", fd)
	s, err := os.Readlink(proc)
	if err != nil {
		return "", err
	}
	return path.Dir(s), nil
}
