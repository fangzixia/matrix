package util

import (
	"bufio"
	"context"
	"io"
)

const defaultEmitChunkSize = 4096

// DefaultEmitChunkSize 为流式工具输出的默认分块大小。
const DefaultEmitChunkSize = defaultEmitChunkSize

// WriteChunks 将字符串按固定大小分块写入 StreamWriter。
func WriteChunks(ctx context.Context, s string, chunkSize int) {
	if s == "" {
		return
	}
	if chunkSize <= 0 {
		chunkSize = defaultEmitChunkSize
	}
	w := StreamWriter(ctx)
	for i := 0; i < len(s); i += chunkSize {
		end := i + chunkSize
		if end > len(s) {
			end = len(s)
		}
		_, _ = w.Write([]byte(s[i:end]))
	}
}

// StreamReader 从 Reader 按块读取并写入 StreamWriter。
func StreamReader(ctx context.Context, r io.Reader, chunkSize int) error {
	if chunkSize <= 0 {
		chunkSize = defaultEmitChunkSize
	}
	w := StreamWriter(ctx)
	br := bufio.NewReaderSize(r, chunkSize)
	buf := make([]byte, chunkSize)
	for {
		n, err := br.Read(buf)
		if n > 0 {
			if _, werr := w.Write(buf[:n]); werr != nil {
				return werr
			}
		}
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
	}
}
