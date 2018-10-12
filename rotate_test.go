package rotate_test

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"syscall"
	"testing"

	"github.com/koorgoo/rotate"
)

var SplitTests = []struct {
	Name string
	Base string
	N    int64
}{
	{"a", "a", 0},
	{"a.1", "a", 1},
	{"a.99", "a", 99},
	{"a.0", "a.0", 0},
	{"a.b", "a.b", 0},
}

func TestSplit(t *testing.T) {
	for _, tt := range SplitTests {
		t.Run(tt.Base, func(t *testing.T) {
			s, n := rotate.Split(tt.Name)
			if s != tt.Base {
				t.Errorf("base: want %q, got %q", tt.Base, s)
			}
			if n != tt.N {
				t.Errorf("n: want %d, got %d", tt.N, n)
			}
		})
	}
}

type ListTest struct {
	Name   string
	Touch  []string
	Result []string
}

func (t *ListTest) String() string {
	return fmt.Sprintf("%s with %v: %v", t.Name, t.Touch, t.Result)
}

var ListTests = []ListTest{
	{
		"a",
		nil,
		[]string{"a"},
	},
	{
		"a",
		[]string{"a.2", "a.1", "a", "a.0"},
		[]string{"a", "a.1", "a.2"}, // sorted, exclude a.0
	},
	{
		"a",
		[]string{"a", "b", "b.1", "c.1", "a.1"},
		[]string{"a", "a.1"},
	},
}

func TestList(t *testing.T) {
	for _, tt := range ListTests {
		t.Run(tt.String(), func(t *testing.T) {
			names := []string{tt.Name}
			names = append(names, tt.Touch...)

			root := touch(t, names...)
			defer os.RemoveAll(root)

			v, err := rotate.List(root, tt.Name)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(v, tt.Result) {
				t.Errorf("want %v, got %v", tt.Result, v)
			}
		})
	}
}

// touch creates files with names and returns a root directory.
func touch(t *testing.T, names ...string) (root string) {
	root, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatalf("touch: %v", err)
	}
	for _, name := range names {
		f, err := open(root, name)
		if err != nil {
			t.Fatalf("touch: %v", err)
		}
		_ = f.Close()
	}
	return
}

// notExist calls t.Fatal() when file exists.
func notExist(t *testing.T, root, name string) {
	_, err := stat(root, name)
	if !os.IsNotExist(err) {
		t.Fatalf("notExist: %v", err)
	}
}

// exist calls t.Fatal() when file does not exist.
func exist(t *testing.T, root, name string) {
	_, err := stat(root, name)
	if err != nil {
		t.Fatalf("exist: %v", err)
	}
}

// inode returns inode number of file.
// It calls t.Fatal() on error.
func inode(t *testing.T, root, name string) uint64 {
	v, err := stat(root, name)
	if err != nil {
		t.Fatalf("inode: %v", err)
	}
	s := v.Sys().(*syscall.Stat_t)
	return s.Ino
}

func ropen(t *testing.T, root, name string, c rotate.Config) (f rotate.File) {
	f, err := open(root, name)
	if err != nil {
		t.Fatalf("ropen: %v", err)
	}
	f, err = rotate.Wrap(f, c)
	if err != nil {
		t.Fatalf("ropen: %v", err)
	}
	return
}

func write(t *testing.T, f rotate.File, s string) (n int) {
	n, err := f.WriteString(s)
	if err != nil {
		if _, ok := err.(*rotate.Error); !ok {
			t.Fatalf("write: %v", err)
		}
	}
	return
}

func remove(t *testing.T, root, name string) {
	err := os.Remove(filepath.Join(root, name))
	if err != nil {
		t.Fatalf("remove: %v", err)
	}
}

func open(root, name string) (*os.File, error) {
	s := filepath.Join(root, name)
	return os.OpenFile(s, rotate.OpenFlag, rotate.OpenPerm)
}

func stat(root, name string) (os.FileInfo, error) {
	return os.Stat(filepath.Join(root, name))
}
