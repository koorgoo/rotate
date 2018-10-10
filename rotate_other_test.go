// +build !linux

package rotate_test

import (
	"testing"

	"github.com/koorgoo/rotate"
)

func TestFile(t *testing.T) {
	root := touch(t, "test")
	f := open(t, root, "test")
	r, err := rotate.Wrap(f, rotate.Config{})
	if err != rotate.ErrNotSupported {
		t.Errorf("want ErrNotSupported, got %v", err)
	}
	if r != nil {
		r.Close()
	}
}
