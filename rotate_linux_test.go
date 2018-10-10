// +build linux

package rotate_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/koorgoo/rotate"
)

// TODO: Write API for testing scenarios.

func TestFile(t *testing.T) {
	root := touch(t, "a")
	defer os.RemoveAll(root)

	name := filepath.Join(root, "a")
	r := rotate.MustOpen(name, rotate.Config{
		Bytes: 5,
		Count: 2,
	})
	defer r.Close()

	n, err := r.Write([]byte("12345"))
	if err != nil {
		t.Fatal(err)
	}
	if n != 5 {
		t.Fatalf("want %d bytes, wrote %d bytes", 5, n)
	}

	// trigger rotation
	_, err = r.Write([]byte("1"))
	if err != nil {
		t.Fatal(err)
	}

	_, err = stat(root, "a.0")
	if err != nil {
		t.Fatal(err)
	}

	_, err = r.Write([]byte("2345"))
	if err != nil {
		t.Fatal(err)
	}

	// trigger rotation
	_, err = r.Write([]byte("1"))
	if err != nil {
		t.Fatal(err)
	}

	_, err = stat(root, "a.1")
	if !os.IsNotExist(err) {
		t.Fatal(err)
	}
}
