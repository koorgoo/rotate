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

// OpenFlag is used to open a file after rotation.
const OpenFlag int = os.O_APPEND | os.O_CREATE | os.O_WRONLY

// Error is returned when rotation fails. It does not cancel write.
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
	// If Bytes == 0, no rotation happens.
	Bytes int64
	// Count defines the maximum amount of files (open + rotated).
	// If Count <= 1, a file will be removed & created on Bytes size.
	Count int64
	// Lock defines whether to lock on write.
	// Must be set for asynchronous writes.
	Lock bool
}

// File is an interface compatible with *os.File.
type File interface {
	io.Writer
	io.Closer

	Fd() uintptr
	Name() string
	Stat() (os.FileInfo, error)
	Sync() error
	WriteString(string) (int, error)
}

// Wrap wraps f with Rotator instance and returns File.
func Wrap(f File, c Config) (File, error) {
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
			mu = new(sync.Mutex)
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

type file struct {
	w     File
	r     Rotator
	mu    mutex
	bytes int64
	n     int64
}

func (f *file) Fd() uintptr                { return f.w.Fd() }
func (f *file) Name() string               { return f.w.Name() }
func (f *file) Stat() (os.FileInfo, error) { return f.w.Stat() }

func (f *file) Sync() (err error) {
	f.mu.Lock()
	err = f.w.Sync()
	f.mu.Unlock()
	return
}

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

func (f *file) WriteString(s string) (int, error) {
	return f.Write([]byte(s))
}

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
	Rotate() (File, error)
}

// Noop return a noop Rotator.
func Noop(f File) Rotator { return &noop{f} }

type noop struct{ f File }

func (n *noop) Rotate() (File, error) { return n.f, nil }

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
		base := filepath.Base(f.Name())
		// save syscall while a single file
		if count < 1 {
			names = []string{base}
			goto AFTER_NAMES
		}
		v, err := List(root, base)
		if err != nil {
			return nil, err
		}
		if len(v) < 1 {
			panic("must contain current file")
		}
		names = make([]string, count)
		copy(names, v)
	}
AFTER_NAMES:
	r = &rotator{
		f:     f,
		mode:  mode,
		root:  root,
		name:  names[0],
		names: names,
	}
	return
}

type rotator struct {
	f     File
	mode  os.FileMode
	root  string
	name  string
	names []string
}

func (r *rotator) abs(name string) string {
	return filepath.Join(r.root, name)
}

func (r *rotator) Rotate() (File, error) {
	err := r.rename()
	if err == nil {
		// TODO: If error, rename file back & remove obsolete `<name>.0` from r.names.
		err = r.reopen()
	}
	return r.f, err
}

func (r *rotator) reopen() error {
	name := r.abs(r.name)
	f, err := os.OpenFile(name, OpenFlag, r.mode)
	if err != nil {
		return err
	}
	// TODO: Handle error.
	_ = r.f.Close()
	r.f = f
	return nil
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
// names must contain at list one item.
//
//     [a]     -> [a.1]
//     [a a.1] -> [a.1 a.2]
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

// SuffixRe is a pattern of rotation counter suffix.
const SuffixRe = `(\.[1-9]+)?$`

var suffixRe = regexp.MustCompile(SuffixRe)

// Split splits name into base part and rotation counter.
// When name cannot be splitted, base equals name.
func Split(name string) (base string, n int64) {
	v := suffixRe.FindStringSubmatch(name)
	if v == nil || v[1] == "" {
		base = name
		return
	}
	base = strings.TrimSuffix(name, v[1])
	n, err := strconv.ParseInt(v[1][1:], 10, 64) // without dot
	if err != nil {
		panic("invalid suffix regexp")
	}
	return
}

// List returns a sorted list of names of existing files which end with SuffixRe.
// If name exists, it is the first item in result.
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
	p, err := regexp.Compile(`^` + name + SuffixRe)
	if err != nil {
		// TODO: Need clearer error message.
		return nil, fmt.Errorf("rotate: %s: %s", name, err)
	}
	return p, nil
}

type mutex interface {
	Lock()
	Unlock()
}

type noMutex struct{}

func (m *noMutex) Lock()   {}
func (m *noMutex) Unlock() {}
