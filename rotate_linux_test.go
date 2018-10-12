// +build linux

package rotate_test

import (
	"os"
	"testing"

	"github.com/koorgoo/rotate"
)

func TestFile_basic(t *testing.T) {
	root := touch(t, "a")
	defer os.RemoveAll(root)

	r := ropen(t, root, "a", rotate.Config{Bytes: 1, Count: 2})
	defer r.Close()

	n := write(t, r, "1")
	if n != 1 {
		t.Fatalf("want 1 byte, wrote %d bytes", n)
	}

	// trigger rotation
	write(t, r, "1")
	exist(t, root, "a.1")

	// trigger rotation
	write(t, r, "1")
	write(t, r, "1")

	notExist(t, root, "a.2")
}

func TestFile_resetsWrittenBytesOnRotation(t *testing.T) {
	root := touch(t, "a")
	defer os.RemoveAll(root)

	r := ropen(t, root, "a", rotate.Config{Bytes: 2, Count: 2})
	defer r.Close()

	// trigger rotation
	write(t, r, "12")
	write(t, r, "1")

	i1 := inode(t, root, "a")
	write(t, r, "2")
	i2 := inode(t, root, "a.1")

	if i1 == i2 {
		t.Fatal("a must not be rotated")
	}
}

func TestFile_recreatesFile(t *testing.T) {
	root := touch(t, "a")
	defer os.RemoveAll(root)

	r := ropen(t, root, "a", rotate.Config{Bytes: 1})
	defer r.Close()

	i1 := inode(t, root, "a")

	// trigger rotation
	write(t, r, "1")
	write(t, r, "1")

	i2 := inode(t, root, "a")
	if i1 == i2 {
		t.Fatal("a must be removed and created")
	}
}

func TestFile_doesNotRenameAllFilesOnError(t *testing.T) {
	// Expected renames:
	//
	// a   a.1    a.2    a.3
	// a  [err]   a.3  [removed]

	root := touch(t, "a", "a.1", "a.2", "a.3")
	defer os.RemoveAll(root)

	r := ropen(t, root, "a", rotate.Config{Bytes: 1, Count: 4})
	defer r.Close()

	remove(t, root, "a.1")

	a2 := inode(t, root, "a.2")

	// trigger rotation
	write(t, r, "1")
	write(t, r, "1")

	exist(t, root, "a")
	notExist(t, root, "a.1")

	a3 := inode(t, root, "a.3")
	if a2 != a3 {
		t.Fatal("a.2 was not renamed to a.3")
	}
}
