package sortutil

import (
	"bufio"
	"container/heap"
	"fmt"
	"io"
	"os"
	"strings"
)

const MaxMemoryBytes = 100 * 1024 * 1024 // 100 MB

type tempFile struct {
	*os.File
	*bufio.Scanner
}

type mergeItem struct {
	line  string
	file  *tempFile
	index int
}

type mergeHeap struct {
	items []mergeItem
	opts  SortOptions
}

func (h *mergeHeap) Len() int { return len(h.items) }
func (h *mergeHeap) Less(i, j int) bool {
	a, b := h.items[i].line, h.items[j].line
	aKey := h.getKey(a)
	bKey := h.getKey(b)

	if h.opts.IgnoreBlanks {
		aKey = trimBlanks(aKey)
		bKey = trimBlanks(bKey)
	}

	var less bool
	if h.opts.Human {
		va, vb := humanValue(aKey), humanValue(bKey)
		if va != vb {
			less = va < vb
		} else {
			less = a < b
		}
	} else if h.opts.Month {
		ma, mb := monthValue(aKey), monthValue(bKey)
		if ma != mb {
			less = ma < mb
		} else {
			less = a < b
		}
	} else if h.opts.Numeric {
		na, nb := numericPrefix(aKey), numericPrefix(bKey)
		if na != nb {
			less = na < nb
		} else {
			less = a < b
		}
	} else {
		less = a < b
	}
	if h.opts.Reverse {
		return !less
	}
	return less
}
func (h *mergeHeap) Swap(i, j int) { h.items[i], h.items[j] = h.items[j], h.items[i] }
func (h *mergeHeap) Push(x any)    { h.items = append(h.items, x.(mergeItem)) }
func (h *mergeHeap) Pop() any {
	old := h.items
	n := len(old)
	x := old[n-1]
	h.items = old[0 : n-1]
	return x
}

func (h *mergeHeap) getKey(line string) string {
	if h.opts.KeyCol <= 0 {
		return line
	}
	fields := strings.Split(line, "\t")
	if h.opts.KeyCol > len(fields) {
		return ""
	}
	return fields[h.opts.KeyCol-1]
}

// ExternalSort performs external merge sort on reader.
func ExternalSort(reader io.Reader, opts SortOptions, initialLines []string) error {
	var tempFiles []*tempFile
	defer func() {
		for _, tf := range tempFiles {
			tf.File.Close()
			os.Remove(tf.File.Name())
		}
	}()

	lines := initialLines
	memoryUsed := estimateMemorySize(lines)

	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		line := scanner.Text()
		lineSize := len(line) + 32

		// Если превысили лимит в памяти - сортируем и сбрасываем порцию
		if memoryUsed+lineSize > MaxMemoryBytes && len(lines) > 0 {
			// Сортируем порцию
			sortedLines := SortInMemory(lines, opts)
			// Пишем во временный файл
			tmpFile, err := createTempFile(sortedLines)
			if err != nil {
				return err
			}
			tempFiles = append(tempFiles, tmpFile)
			lines = nil
			memoryUsed = 0
		}

		lines = append(lines, line)
		memoryUsed += lineSize
	}

	// Проверка ошибки сканера
	if err := scanner.Err(); err != nil {
		return err
	}

	// Последняя порция
	if len(lines) > 0 {
		sortedLines := SortInMemory(lines, opts)
		tmpFile, err := createTempFile(sortedLines)
		if err != nil {
			return err
		}
		tempFiles = append(tempFiles, tmpFile)
	}

	if len(tempFiles) == 0 {
		return nil
	}
	if len(tempFiles) == 1 {
		// Выводим напрямую
		tf := tempFiles[0]
		for tf.Scanner.Scan() {
			fmt.Println(tf.Scanner.Text())
		}
		return tf.Scanner.Err()
	}

	// K-путевое слияние
	return mergeFiles(tempFiles, opts)
}

// mergeFiles performs k-way merge of sorted temp files.
func mergeFiles(files []*tempFile, opts SortOptions) error {
	h := &mergeHeap{
		opts: opts,
	}
	heap.Init(h)

	// Загружаем первую строку из каждого файла
	for i, tf := range files {
		if tf.Scanner.Scan() {
			heap.Push(h, mergeItem{
				line:  tf.Scanner.Text(),
				file:  tf,
				index: i,
			})
		}
	}

	// Запоминаем последнюю выведенную строку для уникальности
	var lastLine string
	first := true

	for h.Len() > 0 {
		item := heap.Pop(h).(mergeItem)
		current := item.line

		// Обработка уникальности (-u)
		shouldPrint := true
		if opts.Unique {
			if first {
				lastLine = current
			} else {
				if equivalent(lastLine, current, opts) {
					shouldPrint = false
				} else {
					lastLine = current
				}
			}
			first = false
		}

		if shouldPrint {
			if _, err := fmt.Println(current); err != nil {
				return err
			}
		}

		// Читаем следующую строку из того же файла
		if item.file.Scanner.Scan() {
			heap.Push(h, mergeItem{
				line:  item.file.Scanner.Text(),
				file:  item.file,
				index: item.index,
			})
		}
	}

	return nil
}

// equivalent checks if two lines are equivalent for -u.
func equivalent(a, b string, opts SortOptions) bool {
	aKey := getKey(a, opts.KeyCol)
	bKey := getKey(b, opts.KeyCol)
	if opts.IgnoreBlanks {
		aKey = trimBlanks(aKey)
		bKey = trimBlanks(bKey)
	}

	if opts.Human {
		return humanValue(aKey) == humanValue(bKey)
	} else if opts.Month {
		return monthValue(aKey) == monthValue(bKey)
	} else if opts.Numeric {
		return numericPrefix(aKey) == numericPrefix(bKey)
	} else {
		return aKey == bKey
	}
}

func createTempFile(lines []string) (*tempFile, error) {
	tmp, err := os.CreateTemp("", "sort-*.tmp")
	if err != nil {
		return nil, err
	}
	for _, line := range lines {
		if _, err = fmt.Fprintln(tmp, line); err != nil {
			tmp.Close()
			return nil, err
		}
	}
	if err = tmp.Close(); err != nil {
		return nil, err
	}

	reopened, err := os.Open(tmp.Name())
	if err != nil {
		return nil, err
	}
	return &tempFile{File: reopened, Scanner: bufio.NewScanner(reopened)}, nil
}

func isUnordered(prev, curr string, opts SortOptions) bool {
	var unordered bool

	if opts.Human {
		prevVal := humanValue(prev)
		currVal := humanValue(curr)
		if prevVal != currVal {
			if opts.Reverse {
				unordered = prevVal < currVal
			} else {
				unordered = prevVal > currVal
			}
		} else {
			if opts.Reverse {
				unordered = prev < curr
			} else {
				unordered = prev > curr
			}
		}
	} else if opts.Month {
		prevMonth := monthValue(prev)
		currMonth := monthValue(curr)
		if prevMonth != currMonth {
			if opts.Reverse {
				unordered = prevMonth < currMonth
			} else {
				unordered = prevMonth > currMonth
			}
		} else {
			if opts.Reverse {
				unordered = prev < curr
			} else {
				unordered = prev > curr
			}
		}
	} else if opts.Numeric {
		prevNum := numericPrefix(prev)
		currNum := numericPrefix(curr)
		if prevNum != currNum {
			if opts.Reverse {
				unordered = prevNum < currNum
			} else {
				unordered = prevNum > currNum
			}
		} else {
			if opts.Reverse {
				unordered = prev < curr
			} else {
				unordered = prev > curr
			}
		}

	} else {
		if opts.Reverse {
			unordered = prev < curr
		} else {
			unordered = prev > curr
		}
	}

	return unordered
}
