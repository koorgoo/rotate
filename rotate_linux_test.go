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

	_, err = stat(root, "a.1")
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

	notExist(t, root, "a.2")
}

func TestFile_doesNotRenameAllFilesOnError(t *testing.T) {
	// Expected renames:
	//
	// a   a.1    a.2    a.3
	// a  [err]   a.3  [removed]

	root := touch(t, "a", "a.1", "a.2", "a.3")
	defer os.RemoveAll(root)

	name := filepath.Join(root, "a")
	r := rotate.MustOpen(name, rotate.Config{Bytes: 1, Count: 4})
	defer r.Close()

	_ = os.Remove(filepath.Join(root, "a.1"))

	a2 := inode(t, root, "a.2")

	_, err := r.WriteString("1")

	// trigger rotation
	_, err = r.WriteString("1")
	if _, ok := err.(*rotate.Error); !ok {
		t.Fatal(err)
	}

	exist(t, root, "a")
	notExist(t, root, "a.1")

	a3 := inode(t, root, "a.3")
	if a2 != a3 {
		t.Fatal("a.2 was not renamed to a.3")
	}
}
