package example

import (
	"bufio"
	"flag"
	"io"

	"github.com/koorgoo/rotate"
)

// Open opens a file using flags.
func Open() rotate.File {
	var name string
	var c rotate.Config

	flag.StringVar(&name, "o", "out", "output file")
	flag.Int64Var(&c.Bytes, "b", 1, "bytes per file")
	flag.Int64Var(&c.Count, "c", 5, "max count of files")
	flag.Parse()

	return rotate.MustOpen(name, c)
}

// Pipe reads line by line from r and write to w.
func Pipe(r io.Reader, w io.Writer) {
	buf := bufio.NewReader(r)

	for {
		b, _, err := buf.ReadLine()
		if err == io.EOF {
			return
		}
		if err != nil {
			panic(err)
		}
		if _, err = w.Write(b); err != nil {
			panic(err)
		}
	}
}
