package main

import (
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/koorgoo/rotate/example"
)

// Writer wrap zap.Logger.
type Writer struct {
	logger *zap.Logger
}

// WriteString implements io.Writer interface.
func (w *Writer) Write(b []byte) (int, error) {
	w.logger.Debug(string(b))
	return len(b), nil
}

func main() {
	r := example.Open()
	defer r.Close()

	logger := zap.New(zapcore.NewCore(
		zapcore.NewConsoleEncoder(zapcore.EncoderConfig{MessageKey: "zap"}),
		r, zapcore.DebugLevel))

	example.Pipe(os.Stdin, &Writer{logger})
}
