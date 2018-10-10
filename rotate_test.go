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
	Base string
	S    string
	N    int64
}{
	{"a", "a", -1},
	{"a.0", "a", 0},
	{"a.99", "a", 99},
}

func TestSplit(t *testing.T) {
	for _, tt := range SplitTests {
		t.Run(tt.Base, func(t *testing.T) {
			s, n := rotate.Split(tt.Base)
			if s != tt.S {
				t.Errorf("want %q, got %q", tt.S, s)
			}
			if n != tt.N {
				t.Errorf("want %d, got %d", tt.N, n)
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
		[]string{"a.1", "a", "a.0"},
		[]string{"a", "a.0", "a.1"}, // sorted
	},
	{
		"a",
		[]string{"a", "b", "b.1", "a.0", "c.1", "a.1"},
		[]string{"a", "a.0", "a.1"},
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
		t.Fatal(err)
	}
	for _, name := range names {
		f := open(t, root, name)
		_ = f.Close()
	}
	return
}

// notExist calls t.Fatal() when file exists.
func notExist(t *testing.T, root, name string) {
	_, err := stat(root, name)
	if !os.IsNotExist(err) {
		t.Fatal(err)
	}
}

// exist calls t.Fatal() when file does not exist.
func exist(t *testing.T, root, name string) {
	_, err := stat(root, name)
	if err != nil {
		t.Fatal(err)
	}
}

// inode returns inode number of file.
// It calls t.Fatal() on error.
func inode(t *testing.T, root, name string) uint64 {
	v, err := stat(root, name)
	if err != nil {
		t.Fatal(err)
	}
	s := v.Sys().(*syscall.Stat_t)
	return s.Ino
}

func open(t *testing.T, root, name string) *os.File {
	s := filepath.Join(root, name)
	f, err := os.OpenFile(s, rotate.OpenFlag, rotate.OpenPerm)
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func stat(root, name string) (os.FileInfo, error) {
	return os.Stat(filepath.Join(root, name))
}
