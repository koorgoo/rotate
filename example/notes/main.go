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
	name  = flag.String("f", "notes.log", "destination")
	bytes = flag.Int64("b", 10, "Config.Bytes")
	count = flag.Int64("c", 5, "Config.Count")
)

func main() {
	flag.Parse()

	f, err := os.OpenFile(*name, rotate.Flags, 0644)
	if err != nil {
		panic(err)
	}
	defer f.Close()

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
		fmt.Print("note: ")
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
