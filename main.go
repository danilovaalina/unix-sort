package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"sort"
	"strconv"
	"strings"
)

var monthMap = map[string]int{
	"Jan": 1,
	"Feb": 2,
	"Mar": 3,
	"Apr": 4,
	"May": 5,
	"Jun": 6,
	"Jul": 7,
	"Aug": 8,
	"Sep": 9,
	"Oct": 10,
	"Nov": 11,
	"Dec": 12,
}

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

	lines, err := readLines(reader)
	if err != nil {
		log.Fatalf("sort: error reading input: %v\n", err)
	}

	if *check {
		for i := 1; i < len(lines); i++ {
			prev := getKey(lines[i-1], *keyCol)
			curr := getKey(lines[i], *keyCol)

			if *ignoreBlanks {
				prev = trimBlanks(prev)
				curr = trimBlanks(curr)
			}

			var unordered bool

			if *human {
				prevVal := humanValue(prev)
				currVal := humanValue(curr)
				if prevVal != currVal {
					if *reverse {
						unordered = prevVal < currVal
					} else {
						unordered = prevVal > currVal
					}
				} else {
					if *reverse {
						unordered = prev < curr
					} else {
						unordered = prev > curr
					}
				}
			} else if *month {
				prevMonth := monthValue(prev)
				currMonth := monthValue(curr)
				if prevMonth != currMonth {
					if *reverse {
						unordered = prevMonth < currMonth
					} else {
						unordered = prevMonth > currMonth
					}
				} else {
					if *reverse {
						unordered = prev < curr
					} else {
						unordered = prev > curr
					}
				}
			} else if *numeric {
				prevNum := numericPrefix(prev)
				currNum := numericPrefix(curr)
				if prevNum != currNum {
					if *reverse {
						unordered = prevNum < currNum
					} else {
						unordered = prevNum > currNum
					}
				} else {
					if *reverse {
						unordered = prev < curr
					} else {
						unordered = prev > curr
					}
				}

			} else {
				if *reverse {
					unordered = prev < curr
				} else {
					unordered = prev > curr
				}
			}

			if unordered {
				fmt.Fprintf(os.Stderr, "sort: %s:%d: disorder: %s\n", source, i+1, lines[i])
				os.Exit(1)
			}
		}

		os.Exit(0)
	}

	sort.SliceStable(lines, func(i, j int) bool {
		a := getKey(lines[i], *keyCol)
		b := getKey(lines[j], *keyCol)
		if *ignoreBlanks {
			a = trimBlanks(a)
			b = trimBlanks(b)
		}
		if *human {
			valA := humanValue(a)
			valB := humanValue(b)
			if valA != valB {
				return valA < valB
			}
			return a < b
		} else if *month {
			monthA := monthValue(a)
			monthB := monthValue(b)
			if monthA != monthB {
				return monthA < monthB
			}
			return a < b
		} else if *numeric {
			numA := numericPrefix(a)
			numB := numericPrefix(b)
			if numA != numB {
				return numA < numB
			}
			return a < b
		}
		return a < b
	})

	if *unique {
		var uniqueLines []string
		if len(lines) > 0 {
			uniqueLines = []string{lines[0]}
			for i := 1; i < len(lines); i++ {
				var prev, curr string
				prev = getKey(lines[i-1], *keyCol)
				curr = getKey(lines[i], *keyCol)
				if *ignoreBlanks {
					prev = trimBlanks(prev)
					curr = trimBlanks(curr)
				}
				var equal bool
				if *human {
					equal = humanValue(prev) == humanValue(curr)
				} else if *month {
					equal = monthValue(prev) == monthValue(curr)
				} else if *numeric {
					equal = numericPrefix(prev) == numericPrefix(curr)
				} else {
					equal = prev == curr
				}
				if !equal {
					uniqueLines = append(uniqueLines, lines[i])
				}
			}
		}
		lines = uniqueLines
	}

	if *reverse {
		for i, j := 0, len(lines)-1; i < j; i, j = i+1, j-1 {
			lines[i], lines[j] = lines[j], lines[i]
		}
	}

	for _, line := range lines {
		fmt.Println(line)
	}
}

func monthValue(s string) int {
	if val, ok := monthMap[s]; ok {
		return val
	}
	return 0
}

func trimBlanks(s string) string {
	return strings.Trim(s, " \t")
}

func getKey(line string, col int) string {
	if col <= 0 {
		return line
	}
	fields := strings.Split(line, "\t")
	if col > len(fields) {
		return ""
	}
	return fields[col-1]
}

// humanValue parses a human-readable number.
func humanValue(s string) float64 {
	number, rest := parseFloat(s)
	if rest == s {
		return 0.0
	}

	// Остаток строки - суффикс
	suffix := strings.TrimSpace(rest)
	if suffix == "" {
		return number
	}

	var (
		multiplier float64
		isBinary   bool
	)

	if len(suffix) >= 2 && suffix[len(suffix)-1] == 'i' {
		isBinary = true
		suffix = suffix[:len(suffix)-1]
	}

	switch suffix {
	case "K":
		multiplier = 1000
	case "M":
		multiplier = 1000 * 1000
	case "G":
		multiplier = 1000 * 1000 * 1000
	case "T":
		multiplier = 1e12
	case "P":
		multiplier = 1e15
	case "E":
		multiplier = 1e18
	default:
		return 0.0
	}

	if isBinary {
		switch multiplier {
		case 1000:
			multiplier = 1024
		case 1e6:
			multiplier = 1024 * 1024
		case 1e9:
			multiplier = 1024 * 1024 * 1024
		case 1e12:
			multiplier = 1024 * 1024 * 1024 * 1024
		case 1e15:
			multiplier = 1 << 50 // ~1.126e15
		}
	}

	return number * multiplier
}

// numericPrefix теперь просто обертка над parseLeadingFloat,
// которая возвращает 0.0, если число не найдено.
func numericPrefix(s string) float64 {
	number, _ := parseFloat(s)
	return number
}

func parseFloat(s string) (float64, string) {
	if len(s) == 0 {
		return 0.0, s
	}

	// Skip leading blanks (space and tab in C locale)
	i := 0
	for i < len(s) && (s[i] == ' ' || s[i] == '\t') {
		i++
	}

	if i == len(s) {
		return 0.0, s // only blanks
	}

	// Optional minus
	start := i
	if s[i] == '-' {
		i++
		if i == len(s) {
			return 0.0, s
		}
	}

	// Digits before decimal point
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
	}

	// Optional decimal point and digits
	if i < len(s) && s[i] == '.' {
		i++
		for i < len(s) && s[i] >= '0' && s[i] <= '9' {
			i++
		}
	}

	if i == start || (i == start+1 && s[start] == '-') {
		// No digits found
		return 0.0, s
	}

	prefix := s[start:i]
	f, err := strconv.ParseFloat(prefix, 64)
	if err != nil {
		return 0.0, s
	}

	return f, s[i:]
}

// readLines reads all lines from the given reader.
func readLines(r io.Reader) ([]string, error) {
	scanner := bufio.NewScanner(r)
	var lines []string
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return lines, nil
}
