// +build !linux

package rotate_test

import (
	"path/filepath"
	"testing"

	"github.com/koorgoo/rotate"
)

func TestFile(t *testing.T) {
	root := touch(t, "test")
	name := filepath.Join(root, "test")
	r, err := rotate.Open(name, rotate.Config{})
	if err != rotate.ErrNotSupported {
		t.Errorf("want ErrNotSupported, got %v", err)
	}
	if r != nil {
		r.Close()
	}
}
