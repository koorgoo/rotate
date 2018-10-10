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

// Common bytes.
const (
	B  int64 = 1
	KB       = 1024 * B
	MB       = 1024 * KB
	GB       = 1024 * MB
)

// Error is returned when rotation fails.
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

// Wrap initializes and returns File.
func Wrap(f *os.File, c Config) (*File, error) {
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
	file := File{
		w:     f,
		r:     r,
		mu:    mu,
		bytes: c.Bytes,
		n:     size,
	}
	return &file, err
}

// File wraps os.File with rotation.
type File struct {
	w     io.WriteCloser
	r     Rotator
	mu    mutex
	bytes int64
	n     int64
}

var _ io.WriteCloser = (*File)(nil)

// Write implements io.Writer interface.
func (f *File) Write(b []byte) (n int, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err = f.rotate(); err != nil {
		return
	}
	n, err = f.w.Write(b)
	f.n += int64(n)
	return
}

// WriteString is like Write, but writes the contents of string s rather than
// a slice of bytes.
func (f *File) WriteString(s string) (int, error) {
	return f.Write([]byte(s))
}

// Close implementes io.Closer interface.
func (f *File) Close() error {
	return f.w.Close()
}

func (f *File) rotate() (err error) {
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

// New returns Rotate for f.
func New(f *os.File, count int64) (Rotator, error) {
	root, err := Dirname(f.Fd())
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
	base := filepath.Base(f.Name())
	names := []string{base}
	{
		// TODO: Rename to List(root, name string) ([]string, error)
		v, err := listRotated(root, base, count)
		if err != nil {
			return nil, err
		}
		names = append(names, v...)
	}
	r := rotator{
		f:     f,
		mode:  mode,
		root:  root,
		names: names,
	}
	return &r, nil
}

func listRotated(root, name string, count int64) ([]string, error) {
	if count <= 1 {
		panic("count must be > 1")
	}

	base := filepath.Base(name)

	var exist []string
	re, err := toRegexp(base)
	if err != nil {
		return nil, err
	}

	err = filepath.Walk(root, func(wpath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() && wpath != root {
			return filepath.SkipDir
		}
		s := filepath.Base(info.Name())
		if re.MatchString(s) {
			exist = append(exist, s)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Strings(exist)

	v := make([]string, count)
	v[0] = base
	copy(v[1:], exist)

	return v, nil
}

func toRegexp(name string) (*regexp.Regexp, error) {
	name = strings.Replace(name, `.`, `\.`, -1)
	p, err := regexp.Compile(`^` + name + `\.\d+$`)
	if err != nil {
		// TODO: Find better error message to pass to user.
		return nil, fmt.Errorf("rotate: %s: %s", name, err)
	}
	return p, nil
}

// Noop return a noop Rotator.
func Noop(f *os.File) Rotator {
	return &noop{f}
}

type noop struct{ f *os.File }

func (n *noop) Rotate() (io.WriteCloser, error) { return n.f, nil }

type rotator struct {
	f     *os.File
	mode  os.FileMode
	root  string
	names []string

	moved bool
}

func (r *rotator) Rotate() (io.WriteCloser, error) {
	if r.names == nil {
		return r.f, nil
	}
	return r.rotate()
}

func (r *rotator) rotate() (*os.File, error) {
	if !r.moved {
		if err := r.move(); err != nil {
			return nil, err
		}
		r.moved = true
	}
	if err := r.reopen(); err == nil {
		r.moved = false
	}
	return r.f, nil
}

func (r *rotator) move() error {
	if err := r.removeLast(); err != nil {
		return err
	}

	rotated := make([]string, len(r.names))
	for i := range r.names {
		if i == 0 {
			rotated[i] = r.names[i] + ".0"
			continue
		}
		if r.names[i] == "" {
			continue
		}
		name, n := splitExt(r.names[i])
		rotated[i] = fmt.Sprintf("%s.%d", name, n+1)
	}

	var i int
	var err error

	for i = len(r.names) - 1; i >= 0; i-- {
		if r.names[i] == "" {
			continue
		}
		src := filepath.Join(r.root, r.names[i])
		dest := filepath.Join(r.root, rotated[i])
		if _, err := os.Stat(dest); !os.IsNotExist(err) {
			_ = os.Remove(dest)
		}
		err = os.Rename(src, dest)
		if err != nil {
			break
		}
	}
	if err != nil {
		for j := i + 1; j < len(r.names); j++ {
			r.names[j] = ""
		}
		return err
	}
	// BUG: Names of not moved files are changed.
	for j := 1; j < i; j++ {
		r.names[i] = rotated[i-1]
	}
	return nil
}

func (r *rotator) removeLast() (err error) {
	i := len(r.names) - 1
	if r.names[i] == "" {
		return
	}
	err = os.Remove(r.names[i])
	if err == nil {
		r.names[i] = ""
	}
	return
}

func (r *rotator) reopen() error {
	name := filepath.Join(r.root, r.names[0])
	f, err := os.OpenFile(name, OpenFlag, r.mode)
	if err != nil {
		return err
	}
	// TODO: Handle error.
	_ = r.f.Close()
	r.f = f
	return nil
}

func splitExt(name string) (string, int64) {
	var sep int
	runes := []rune(name)
	for i, v := range runes {
		if v == rune('.') {
			sep = i
		}
	}
	base := string(runes[:sep])
	ext := string(runes[sep+1:])
	n, err := strconv.ParseInt(ext, 10, 64)
	if err != nil {
		panic("invalid extension: " + name)
	}
	return base, n
}

type mutex interface {
	Lock()
	Unlock()
}

type noMutex struct{}

func (m *noMutex) Lock()   {}
func (m *noMutex) Unlock() {}
