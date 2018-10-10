package rotate

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

var ToRegexpTests = []struct {
	Name   string
	String string
}{
	{"file.txt", `^file\.txt\.\d+$`},
}

func TestToRegexp(t *testing.T) {
	for _, tt := range ToRegexpTests {
		t.Run(tt.Name, func(t *testing.T) {
			re, err := toRegexp(tt.Name)
			if err != nil {
				t.Fatal(err)
			}
			if tt.String != re.String() {
				t.Errorf("want %q, got %q", tt.String, re.String())
			}
		})
	}
}

type RotatedTest struct {
	Name   string
	Count  int64
	Exist  []string
	Result []string
}

func (t *RotatedTest) String() string {
	return fmt.Sprintf("%s(%d): %v from %v", t.Name, t.Count, t.Result, t.Exist)
}

var RotatedTests = []RotatedTest{
	// limit by count
	{
		"a",
		2,
		[]string{"a", "a.0", "a.1"},
		[]string{"a", "a.0"},
	},
	{
		"a",
		3,
		[]string{"a", "a.0", "a.1"},
		[]string{"a", "a.0", "a.1"},
	},
	{
		"a",
		5,
		[]string{"a", "a.0", "a.1"},
		[]string{"a", "a.0", "a.1", "", ""},
	},
	// filter by prefix
	{
		"a",
		2,
		[]string{"a", "b", "b.1", "a.0", "c.1", "a.1"},
		[]string{"a", "a.0"},
	},
}

func TestListRotated(t *testing.T) {
	for _, tt := range RotatedTests {
		t.Run(tt.String(), func(t *testing.T) {
			names := make([]string, len(tt.Exist)+1)
			names[0] = tt.Name
			copy(names[1:], tt.Exist)

			root := touch(t, names...)
			defer os.RemoveAll(root)

			v, err := listRotated(root, tt.Name, tt.Count)
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
