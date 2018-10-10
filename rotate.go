package rotate

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
)

// ErrNotSupported is returned when rotation is not supported on a current system.
var ErrNotSupported = fmt.Errorf("rotate: not supported on %s", runtime.GOOS)

// File constants.
const (
	OpenFlag int         = os.O_APPEND | os.O_CREATE | os.O_WRONLY
	OpenPerm os.FileMode = 0644
)

// Error is returned when rotation fails.
// When Error, bytes are written to old file.
type Error struct {
	Filename string
	Err      error
}

func (e *Error) Error() string {
	return fmt.Sprintf("rotate: %v", e.Err)
}

// Config defines rotating policy.
type Config struct {
	// Bytes sets soft limit for file size.
	// Soft limit may be exceeded to write a message to a single file.
	// No rotating will be done with 0 Bytes.
	Bytes int64
	// Count defines the maximum amount of files - 1 current + (Count-1) rotated.
	// A Count of 0 means no limit for rotated files.
	Count int64
	// Lock defines whether to lock on write.
	// Must be set for asynchronous writes.
	Lock bool
}

// File is an interface of *os.File.
//
// It was introduced for testing.
type File interface {
	Fd() uintptr
	Name() string
	Stat() (os.FileInfo, error)
	io.WriteCloser
}

// WriteCloser wraps io.WriteCloser adding WriteString() shortcut.
type WriteCloser interface {
	io.WriteCloser
	WriteString(string) (int, error)
}

// Wrap initializes and returns File.
func Wrap(f File, c Config) (WriteCloser, error) {
	r, err := New(f, c.Count)
	if err != nil && err != ErrNotSupported {
		return nil, err
	}
	var size int64
	{
		v, err := f.Stat()
		if err != nil {
			return nil, err
		}
		size = v.Size()
	}
	var mu mutex
	{
		if c.Lock {
			mu = &sync.Mutex{}
		} else {
			mu = new(noMutex)
		}
	}
	ff := file{
		w:     f,
		r:     r,
		mu:    mu,
		bytes: c.Bytes,
		n:     size,
	}
	return &ff, err
}

// file wraps os.File with rotation.
type file struct {
	w     io.WriteCloser
	r     Rotator
	mu    mutex
	bytes int64
	n     int64
}

var _ io.WriteCloser = (*file)(nil)

// Write implements io.Writer interface.
func (f *file) Write(b []byte) (n int, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	rerr := f.rotate()
	n, err = f.w.Write(b)
	if err == nil {
		err = rerr
	}
	f.n += int64(n)
	return
}

// WriteString is like Write, but writes the contents of string s rather than
// a slice of bytes.
func (f *file) WriteString(s string) (int, error) {
	return f.Write([]byte(s))
}

// Close implementes io.Closer interface.
func (f *file) Close() error {
	return f.w.Close()
}

func (f *file) rotate() (err error) {
	if f.bytes <= 0 || f.n < f.bytes {
		return nil
	}
	f.w, err = f.r.Rotate()
	return
}

// Rotator is an interface for file rotation.
type Rotator interface {
	Rotate() (io.WriteCloser, error)
}

// dirnamer is a testing interface.
type dirnamer interface {
	Dirname() string
}

// New returns Rotator for f.
func New(f File, count int64) (r Rotator, err error) {
	var root string
	if v, ok := f.(dirnamer); ok {
		root = v.Dirname()
	} else {
		root, err = Dirname(f.Fd())
	}
	if err == ErrNotSupported {
		return Noop(f), err
	}
	if err != nil {
		return nil, err
	}
	if count <= 1 {
		return Noop(f), nil
	}
	var mode os.FileMode
	{
		v, err := f.Stat()
		if err != nil {
			return nil, err
		}
		mode = v.Mode()
	}
	var names []string
	{
		v, err := List(root, f.Name())
		if err != nil {
			return nil, err
		}
		if len(v) < 1 {
			panic("must contain current file")
		}
		names = make([]string, count)
		copy(names, v)
	}
	r = &rotator{
		f:     f,
		mode:  mode,
		root:  root,
		names: names,
	}
	return
}

// List returns a sorted list of files matching rotation pattern `^<name>(\.\d+)?$`.
func List(root, name string) ([]string, error) {
	base := filepath.Base(name)

	re, err := toRegexp(base)
	if err != nil {
		return nil, err
	}

	var names []string
	err = filepath.Walk(root, func(wpath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() && wpath != root {
			return filepath.SkipDir
		}
		s := filepath.Base(info.Name())
		if re.MatchString(s) {
			names = append(names, s)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Strings(names)
	return names, nil
}

func toRegexp(name string) (*regexp.Regexp, error) {
	name = strings.Replace(name, `.`, `\.`, -1)
	p, err := regexp.Compile(`^` + name + `(\.\d+)?$`)
	if err != nil {
		// TODO: Need clearer error message.
		return nil, fmt.Errorf("rotate: %s: %s", name, err)
	}
	return p, nil
}

// Noop return a noop Rotator.
func Noop(w io.WriteCloser) Rotator {
	return &noop{w}
}

type noop struct{ w io.WriteCloser }

func (n *noop) Rotate() (io.WriteCloser, error) { return n.w, nil }

type rotator struct {
	f     File
	mode  os.FileMode
	root  string
	names []string
}

func (r *rotator) abs(name string) string {
	return filepath.Join(r.root, name)
}

func (r *rotator) Rotate() (io.WriteCloser, error) {
	err := r.rename()
	if err == nil {
		// TODO: If error, rename file back & remove obsolete `<name>.0` from r.names.
		err = r.reopen()
	}
	return r.f, err
}

func (r *rotator) rename() (err error) {
	if s := r.names[len(r.names)-1]; s != "" {
		err = os.Remove(r.abs(s))
		if err != nil {
			return &Error{
				Filename: s,
				Err:      err,
			}
		}
		r.names[len(r.names)-1] = ""
	}

	names := shift(r.names)

	var i int
	for i = len(r.names) - 1; i >= 0; i-- {
		if r.names[i] == "" {
			continue
		}
		err = os.Rename(
			r.abs(r.names[i]),
			r.abs(names[i]),
		)
		if err != nil {
			err = &Error{
				Filename: r.names[i],
				Err:      err,
			}
			break
		}
	}

	if i == -1 { // renamed all
		copy(r.names[1:], names)
	} else {
		// leave last empty (removed)
		copy(r.names[i+1:len(names)-2], names[i+1:])
	}
	return
}

// shift returns a list of names with incremented rotation suffix.
// v must contain at list one item.
//
//     [a]     -> [a.0]
//     [a a.0] -> [a.0 a.1]
//
func shift(names []string) []string {
	t := make([]string, len(names))
	for i, s := range names {
		if s == "" {
			break
		}
		base, n := Split(s)
		t[i] = fmt.Sprintf("%s.%d", base, n+1)
	}
	return t
}

var sufRe = regexp.MustCompile(`\.(\d+)?$`)

// Split splits base name into a cleaned one and rotation counter.
// If name has no rotation suffix, n equals -1.
func Split(base string) (s string, n int64) {
	s, n = base, -1
	v := sufRe.FindStringSubmatch(base)
	if v == nil {
		s, n = base, -1
		return
	}
	var err error
	s = strings.TrimSuffix(base, "."+v[1])
	n, err = strconv.ParseInt(v[1], 10, 64)
	if err != nil {
		panic(fmt.Sprintf("unexpected name: %s: %v", base, v))
	}
	return
}

// rename moves oldpath to newpath.
// Set removeNew to try to remove newpath (+1 excess syscall when does not exist).
func rename(oldpath, newpath string, removeNew bool) error {
	if removeNew {
		err := os.Remove(newpath)
		if err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return os.Rename(oldpath, newpath)
}

func (r *rotator) reopen() error {
	name := r.abs(r.names[0])
	f, err := os.OpenFile(name, OpenFlag, r.mode)
	if err != nil {
		return err
	}
	// TODO: Handle error.
	_ = r.f.Close()
	r.f = f
	return nil
}

type mutex interface {
	Lock()
	Unlock()
}

type noMutex struct{}

func (m *noMutex) Lock()   {}
func (m *noMutex) Unlock() {}
