package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/koorgoo/rotate"
)

var (
	name  = flag.String("name", "notes.log", "destination")
	bytes = flag.Int64("b", 100, "Config.Bytes")
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
	for {
		fmt.Print("note: ")
		if _, err = fmt.Scanln(&note); err != nil {
			panic(err)
		}
		_, err = r.WriteString(note + "\n")
		if err != nil {
			fmt.Println(err)
		}
	}
}
