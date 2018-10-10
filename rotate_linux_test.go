// +build linux

package rotate_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/koorgoo/rotate"
)

// TODO: Write API for testing scenarios.

func TestFile_basic(t *testing.T) {
	root := touch(t, "a")
	defer os.RemoveAll(root)

	name := filepath.Join(root, "a")
	r := rotate.MustOpen(name, rotate.Config{Bytes: 1, Count: 2})
	defer r.Close()

	n, err := r.WriteString("1")
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("want 1 byte, wrote %d bytes", n)
	}

	// trigger rotation
	_, err = r.WriteString("1")
	if err != nil {
		t.Fatal(err)
	}

	_, err = stat(root, "a.0")
	if err != nil {
		t.Fatal(err)
	}

	_, err = r.WriteString("1")
	if err != nil {
		t.Fatal(err)
	}

	// trigger rotation
	_, err = r.WriteString("1")
	if err != nil {
		t.Fatal(err)
	}

	notExist(t, root, "a.1")
}

func TestFile_partialRename(t *testing.T) {
	root := touch(t, "a", "a.0", "a.1", "a.2")
	defer os.RemoveAll(root)

	name := filepath.Join(root, "a")
	r := rotate.MustOpen(name, rotate.Config{Bytes: 1, Count: 4})
	defer r.Close()

	// will cause error while renaming to a.1
	_ = os.Remove(filepath.Join(root, "a.0"))

	a1 := inode(t, root, "a.1")

	_, err := r.WriteString("1")

	// trigger rotation
	_, err = r.WriteString("1")
	if _, ok := err.(*rotate.Error); !ok {
		t.Fatal(err)
	}

	notExist(t, root, "a.1")
	exist(t, root, "a.2")

	a2 := inode(t, root, "a.2")
	if a1 != a2 {
		t.Fatal("a.1: was not renamed")
	}
}
