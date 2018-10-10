package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/koorgoo/rotate"
)

var (
	name  = flag.String("o", "log", "output file")
	bytes = flag.Int64("b", 10, "bytes per file")
	count = flag.Int64("c", 5, "max count of files")
)

func main() {
	flag.Parse()

	f, err := os.OpenFile(*name, rotate.Flags, 0644)
	if err != nil {
		panic(err)
	}
	defer f.Close()

	fmt.Printf("*** Text is written to %q\n", *name)
	fmt.Printf("*** After %d bytes the file is rotated\n", *bytes)
	fmt.Println()

	r, err := rotate.Wrap(f, rotate.Config{
		Bytes: *bytes,
		Count: *count,
	})
	if err != nil {
		panic(err)
	}

	var note string
	reader := bufio.NewReader(os.Stdin)

	var exit bool

	for !exit {
		note, err = reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				exit = true
			} else {
				panic(err)
			}
		}
		_, err = r.WriteString(note)
		if err != nil {
			fmt.Println(err)
		}
	}
}
