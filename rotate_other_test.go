// +build !linux

package rotate

import (
	"path/filepath"
	"testing"
)

func TestFile(t *testing.T) {
	root := touch(t, "test")
	r, err := Open(filepath.Join(root, "test"), Config{})
	if err != ErrNotSupported {
		t.Errorf("want ErrNotSupported, got %v", err)
	}
	if r != nil {
		r.Close()
	}
}
