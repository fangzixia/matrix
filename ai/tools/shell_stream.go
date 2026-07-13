package tools

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"matrix/ai/util"
	"os/exec"
	"strings"
)

// runShellStreamed 执行 shell 命令并将 stdout/stderr 合并流式推送。
func runShellStreamed(ctx context.Context, cmd *exec.Cmd) (string, error) {
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", err
	}
	cmd.Stderr = cmd.Stdout
	if err := cmd.Start(); err != nil {
		return "", err
	}
	var sb strings.Builder
	br := bufio.NewReader(stdout)
	buf := make([]byte, defaultEmitChunkSize)
	w := util.StreamWriter(ctx)
	for {
		n, readErr := br.Read(buf)
		if n > 0 {
			chunk := string(buf[:n])
			sb.WriteString(chunk)
			_, _ = w.Write(buf[:n])
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			_ = cmd.Wait()
			return sb.String(), readErr
		}
	}
	waitErr := cmd.Wait()
	result := sb.String()
	if waitErr != nil {
		return result, fmt.Errorf("命令失败: %w", waitErr)
	}
	return result, nil
}
