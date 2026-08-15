package index

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
)

func readCompleteLines(
	path string,
	start int64,
	handle func([]byte),
) (parsedOffset int64, bytesRead int64, err error) {
	file, err := os.Open(path)
	if err != nil {
		return start, 0, fmt.Errorf("open %q: %w", path, err)
	}
	defer file.Close()

	if _, err := file.Seek(start, io.SeekStart); err != nil {
		return start, 0, fmt.Errorf("seek %q to %d: %w", path, start, err)
	}

	parsedOffset = start
	reader := bufio.NewReader(file)
	for {
		line, readErr := reader.ReadBytes('\n')
		bytesRead += int64(len(line))
		if len(line) != 0 && line[len(line)-1] == '\n' {
			handle(line)
			parsedOffset += int64(len(line))
		}

		switch {
		case readErr == nil:
			continue
		case errors.Is(readErr, io.EOF):
			return parsedOffset, bytesRead, nil
		default:
			return parsedOffset, bytesRead, fmt.Errorf("read %q: %w", path, readErr)
		}
	}
}
