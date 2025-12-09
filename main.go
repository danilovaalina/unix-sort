package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"

	"unix-sort/sortutil"
)

func main() {
	reverse := flag.Bool("r", false, "sort in reverse order")
	numeric := flag.Bool("n", false, "sort numerically")
	unique := flag.Bool("u", false, "suppress duplicate lines")
	keyCol := flag.Int("k", 0, "sort by column N (1-based index)")
	ignoreBlanks := flag.Bool("b", false, "ignore leading and trailing blanks")
	check := flag.Bool("c", false, "check whether input is sorted")
	month := flag.Bool("M", false, "sort by month name")
	human := flag.Bool("h", false, "sort by human-readable numeric values")

	flag.Parse()

	source := "-"
	var reader io.Reader = os.Stdin
	if flag.NArg() > 0 {
		source = flag.Arg(0)
		file, err := os.Open(source)
		if err != nil {
			log.Fatalf("sort: cannot open '%s': %v\n", source, err)
		}
		defer func() { _ = file.Close() }()
		reader = file
	}

	opts := sortutil.SortOptions{
		Reverse:      *reverse,
		Numeric:      *numeric,
		Month:        *month,
		Human:        *human,
		KeyCol:       *keyCol,
		IgnoreBlanks: *ignoreBlanks,
		Unique:       *unique,
	}

	if *check {
		err := sortutil.CheckSorting(reader, source, opts)
		if err != nil {
			log.Fatal(err)
		}
		return
	}

	// Попытка in-memory сортировки
	lines, err := sortutil.ReadLinesWithLimit(reader, sortutil.MaxMemoryBytes)
	if err != nil {
		if errors.Is(err, sortutil.ErrInputTooLarge) {
			err = sortutil.ExternalSort(reader, opts, lines)
			if err != nil {
				log.Fatal(err)
			}
			return
		}
		log.Fatal(err)
	}

	lines = sortutil.SortInMemory(lines, opts)
	for _, line := range lines {
		fmt.Println(line)
	}
}
