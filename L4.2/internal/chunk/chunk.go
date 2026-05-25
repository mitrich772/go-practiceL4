// Package chunk отвечает за разбиение входного потока на непрерывные
// фрагменты, которые отправляются разным серверам.
package chunk

import "strings"

// Chunk — кусок входных данных вместе с номером первой строки в исходнике
// (1-based). Это позволяет серверу формировать корректные номера строк для
// флага -n даже при произвольном порядке чанков.
type Chunk struct {
	StartLine int
	Data      string
}

// Split режет вход на n непрерывных чанков примерно равной длины (по строкам).
func Split(data string, n int) []Chunk {
	if n <= 0 {
		n = 1
	}

	normalized := strings.ReplaceAll(data, "\r\n", "\n")

	if strings.HasSuffix(normalized, "\n") {
		normalized = normalized[:len(normalized)-1]
	}

	var lines []string
	if normalized != "" {
		lines = strings.Split(normalized, "\n")
	}

	total := len(lines)
	chunks := make([]Chunk, n)

	if total == 0 {
		for i := range chunks {
			chunks[i] = Chunk{StartLine: 1, Data: ""}
		}
		return chunks
	}

	// Базовый размер чанка и остаток, распределяемый по первым r чанкам,
	// чтобы избежать сильного перекоса на маленьких входах.
	base := total / n
	rem := total % n

	start := 0
	startLine := 1
	for i := 0; i < n; i++ {
		size := base
		if i < rem {
			size++
		}
		end := start + size
		if end > total {
			end = total
		}

		var data string
		if size > 0 {
			data = strings.Join(lines[start:end], "\n")
		}
		chunks[i] = Chunk{StartLine: startLine, Data: data}

		startLine += size
		start = end
	}

	return chunks
}
