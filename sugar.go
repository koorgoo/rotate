package rotate

import "os"

// Bytes.
const (
	B  int64 = 1
	KB       = 1024 * B
	MB       = 1024 * KB
	GB       = 1024 * MB
)

// MustWrap is like Wrap, but panics on error. ErrNotSupported is skipped.
func MustWrap(f File, c Config) File {
	r, err := Wrap(f, c)
	if mustPanic(err) {
		panic(err)
	}
	return r
}

func mustPanic(err error) bool {
	if err == ErrNotSupported {
		return false
	}
	return err != nil
}

// MustOpen is like Open, but panic on error. ErrNotSupported is skipped.
func MustOpen(name string, c Config) File {
	f, err := Open(name, c)
	if mustPanic(err) {
		panic(err)
	}
	return f
}

// Open opens a file and wraps it.
func Open(name string, c Config) (File, error) {
	f, err := os.OpenFile(name, OpenFlag, OpenPerm)
	if err != nil {
		return nil, err
	}
	r, err := Wrap(f, c)
	if err == ErrNotSupported {
		return r, err
	}
	if err != nil {
		_ = f.Close()
	}
	return r, err
}
