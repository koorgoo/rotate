package rotate

import (
	"fmt"
	"os"
	"path"
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

// Flags are used on reopen.
const Flags = os.O_APPEND | os.O_CREATE | os.O_WRONLY

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
	info, err2 := f.Stat()
	if err2 != nil {
		return nil, err2
	}
	var mu mutex = &noMutex{}
	if c.Lock {
		mu = &sync.Mutex{}
	}
	file := File{
		File:  f,
		r:     r,
		mu:    mu,
		bytes: c.Bytes,
		n:     info.Size(),
	}
	// allow ErrNotSupported
	return &file, err
}

// File wraps os.File with rotation.
type File struct {
	*os.File
	r     Rotator
	mu    mutex
	bytes int64
	n     int64
}

func (f *File) Write(b []byte) (n int, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err = f.rotate(); err != nil {
		return
	}
	n, err = f.File.Write(b)
	f.n += int64(n)
	return
}

func (f *File) rotate() (err error) {
	if f.bytes <= 0 || f.n < f.bytes {
		return nil
	}
	f.File, err = f.r.Rotate()
	return
}

// Rotator is an interface for file rotation.
type Rotator interface {
	Rotate() (*os.File, error)
}

// New returns Rotate for f.
func New(f *os.File, count int64) (Rotator, error) {
	if count <= 1 {
		return &Noop{f}, nil
	}
	root, err := Dirname(f.Fd())
	if err == ErrNotSupported {
		return &Noop{f}, err
	}
	if err != nil {
		return nil, err
	}
	names, err := listRotated(root, f.Name(), count)
	if err != nil {
		return nil, err
	}
	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	r := rotator{
		f:     f,
		mode:  info.Mode(),
		root:  root,
		names: names,
	}
	return &r, nil
}

func listRotated(root, name string, count int64) ([]string, error) {
	if count <= 1 {
		panic("count must be > 1")
	}

	var exist []string
	re, err := toRegexp(name)
	if err != nil {
		return nil, err
	}

	err = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() && path != root {
			return filepath.SkipDir
		}
		if re.MatchString(info.Name()) {
			exist = append(exist, info.Name())
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	v := make([]string, 1, count)
	v[0] = name

	sort.Strings(exist)
	for i := range exist {
		if len(v) < cap(v) {
			v = append(v, exist[i])
		}
	}
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

// Noop does not rotation.
// It is returned with ErrNotSupported.
type Noop struct{ f *os.File }

// Rotate implements Rotator interface.
func (n *Noop) Rotate() (*os.File, error) { return n.f, nil }

type rotator struct {
	f     *os.File
	mode  os.FileMode
	root  string
	names []string

	moved bool
}

func (r *rotator) Rotate() (*os.File, error) {
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
		if r.names[i] != "" {
			name, n := splitExt(r.names[i])
			rotated[i] = fmt.Sprintf("%s.%d", name, n+1)
		}
	}

	var i int
	var err error

	for i = len(r.names) - 1; i >= 0; i-- {
		if r.names[i] == "" {
			continue
		}
		err = os.Rename(
			path.Join(r.root, r.names[i]),
			path.Join(r.root, rotated[i]),
		)
		if err != nil {
			break
		}
	}
	if err != nil {
		for ; i < len(r.names); i++ {
			r.names[i] = ""
		}
		return err
	}
	for i := 1; i < len(r.names); i++ {
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
	f, err := os.OpenFile(r.names[0], Flags, r.mode)
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
