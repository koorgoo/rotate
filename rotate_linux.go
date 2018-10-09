// +build linux

package rotate

import (
	"fmt"
	"os"
	"path"
)

// Dirname returns directory name containing fd.
func Dirname(fd uintptr) (string, error) {
	pid := os.Getpid()
	proc := fmt.Sprintf("/proc/%d/fd/%d", pid, fd)
	s, err := os.Readlink(proc)
	if err != nil {
		return "", err
	}
	return path.Dir(s), nil
}
