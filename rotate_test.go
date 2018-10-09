package rotate

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
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

func (t *RotatedTest) PrepareDir() (string, error) {
	dir, err := ioutil.TempDir("", "")
	if err != nil {
		return "", err
	}
	if err := touch(dir, t.Name); err != nil {
		return "", err
	}
	for _, name := range t.Exist {
		if err := touch(dir, name); err != nil {
			return "", err
		}
	}
	return dir, nil
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
		10,
		[]string{"a", "a.0", "a.1"},
		[]string{"a", "a.0", "a.1"},
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
			root, err := tt.PrepareDir()
			if err != nil {
				t.Fatal(err)
			}
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

func touch(dir, name string) error {
	s := path.Join(dir, name)
	f, err := os.OpenFile(s, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	f.Close()
	return nil
}
