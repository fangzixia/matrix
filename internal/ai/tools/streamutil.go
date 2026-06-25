package tools

import (
	"bufio"
	"context"
	"io"
)

const defaultEmitChunkSize = 4096

// EmitChunks 将字符串按固定大小分块推送流式输出。
func EmitChunks(ctx context.Context, s string, chunkSize int) {
	if s == "" {
		return
	}
	if chunkSize <= 0 {
		chunkSize = defaultEmitChunkSize
	}
	for i := 0; i < len(s); i += chunkSize {
		end := i + chunkSize
		if end > len(s) {
			end = len(s)
		}
		EmitOutput(ctx, s[i:end])
	}
}

// StreamReader 从 Reader 按行或块读取并推送流式输出。
func StreamReader(ctx context.Context, r io.Reader, chunkSize int) error {
	if chunkSize <= 0 {
		chunkSize = defaultEmitChunkSize
	}
	br := bufio.NewReaderSize(r, chunkSize)
	buf := make([]byte, chunkSize)
	for {
		n, err := br.Read(buf)
		if n > 0 {
			EmitOutput(ctx, string(buf[:n]))
		}
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
	}
}
