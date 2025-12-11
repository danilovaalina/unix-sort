package sortutil

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
)

var ErrInputTooLarge = errors.New("input too large for in-memory sort")

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

type SortOptions struct {
	Reverse      bool
	Numeric      bool
	Month        bool
	Human        bool
	KeyCol       int
	IgnoreBlanks bool
	Unique       bool
}

// ReadLinesWithLimit reads lines from r until memory limit is reached.
// Returns error if input exceeds maxBytes (and at least one line was read).
func ReadLinesWithLimit(s *bufio.Scanner) ([]string, error) {
	var lines []string
	totalSize := 0

	for s.Scan() {
		line := scanner.Text()
		// Оценка памяти: длина строки + накладные расходы среза и строки
		lineSize := len(line) + 16
		if totalSize+lineSize > maxMemoryBytes {
			lines = append(lines, line)
			return lines, ErrInputTooLarge
		}
		lines = append(lines, line)
		totalSize += lineSize
	}

	if err := s.Err(); err != nil {
		return nil, err
	}
	return lines, nil
}

// estimateMemorySize returns an approximate memory footprint of a []string in bytes.
func estimateMemorySize(lines []string) int {
	size := 0
	for _, s := range lines {
		// len(s) — байты данных строки
		// +16 — overhead на строку в Go (header + выравнивание)
		size += len(s) + 16
	}
	return size
}

func SortInMemory(lines []string, opts SortOptions) []string {
	sort.SliceStable(lines, func(i, j int) bool {
		a := getKey(lines[i], opts.KeyCol)
		b := getKey(lines[j], opts.KeyCol)
		if opts.IgnoreBlanks {
			a = trimBlanks(a)
			b = trimBlanks(b)
		}
		if opts.Human {
			valA := humanValue(a)
			valB := humanValue(b)
			if valA != valB {
				return valA < valB
			}
			return a < b
		} else if opts.Month {
			monthA := monthValue(a)
			monthB := monthValue(b)
			if monthA != monthB {
				return monthA < monthB
			}
			return a < b
		} else if opts.Numeric {
			numA := numericPrefix(a)
			numB := numericPrefix(b)
			if numA != numB {
				return numA < numB
			}
			return a < b
		}
		return a < b
	})

	if opts.Unique {
		var uniqueLines []string
		if len(lines) > 0 {
			uniqueLines = []string{lines[0]}
			for i := 1; i < len(lines); i++ {
				prev := getKey(lines[i-1], opts.KeyCol)
				curr := getKey(lines[i], opts.KeyCol)
				if opts.IgnoreBlanks {
					prev = trimBlanks(prev)
					curr = trimBlanks(curr)
				}
				equal := false
				if opts.Human {
					equal = humanValue(prev) == humanValue(curr)
				} else if opts.Month {
					equal = monthValue(prev) == monthValue(curr)
				} else if opts.Numeric {
					equal = numericPrefix(prev) == numericPrefix(curr)
				} else {
					equal = prev == curr
				}
				if !equal {
					uniqueLines = append(uniqueLines, lines[i])
				}
			}
			lines = uniqueLines
		}
	}

	if opts.Reverse {
		for i, j := 0, len(lines)-1; i < j; i, j = i+1, j-1 {
			lines[i], lines[j] = lines[j], lines[i]
		}
	}

	return lines
}

func CheckSorting(s *bufio.Scanner, source string, opts SortOptions) error {
	prevLine := s.Text()

	lineNum := 2
	for s.Scan() {
		currLine := s.Text()
		prev := getKey(prevLine, opts.KeyCol)
		curr := getKey(currLine, opts.KeyCol)
		if opts.IgnoreBlanks {
			prev = trimBlanks(prev)
			curr = trimBlanks(curr)
		}
		unordered := isUnordered(prev, curr, opts)
		if unordered {
			fmt.Fprintf(os.Stderr, "sort: %s:%d: disorder: %s\n", source, lineNum, currLine)
			os.Exit(1)
		}

		prevLine = currLine
		lineNum++
	}
	return nil
}

func monthValue(s string) int {
	if val, ok := monthMap[s]; ok {
		return val
	}
	return 0
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
