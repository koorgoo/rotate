package main

import (
	"os"

	"github.com/sirupsen/logrus"

	"github.com/koorgoo/rotate/example"
)

// Writer wraps logrus.Logger.
type Writer struct {
	logger *logrus.Logger
}

// Write implements io.Writer interface.
func (w *Writer) Write(b []byte) (int, error) {
	w.logger.Debug(string(b))
	return len(b), nil
}

func main() {
	r := example.Open()
	defer r.Close()

	logger := logrus.New()
	logger.SetOutput(r)
	logger.SetLevel(logrus.DebugLevel)

	example.Pipe(os.Stdin, &Writer{logger})
}
