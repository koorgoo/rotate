package rotate

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

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

			v, err := List(root, tt.Name)
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
		f, err := open(root, name)
		if err != nil {
			t.Fatal(err)
		}
		_ = f.Close()
	}
	return
}

func open(root, name string) (*os.File, error) {
	s := filepath.Join(root, name)
	return os.OpenFile(s, OpenFlag, OpenPerm)
}

func stat(root, name string) (os.FileInfo, error) {
	return os.Stat(filepath.Join(root, name))
}
