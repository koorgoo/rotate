package main

import (
	"log"
	"os"

	"github.com/koorgoo/rotate/example"
)

// Writer wraps log.Logger.
type Writer struct {
	logger *log.Logger
}

// Write implements io.Writer interface.
func (w *Writer) Write(b []byte) (int, error) {
	w.logger.Println(string(b))
	return len(b), nil
}

func main() {
	r := example.Open()
	defer r.Close()

	logger := log.New(r, "", 0)

	example.Pipe(os.Stdin, &Writer{logger})
}
