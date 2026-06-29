package logging

import (
	"context"
	"encoding/json"
	"io"
	"sync"
	"time"
)

type jsonLineWriter struct {
	w  io.Writer
	mu sync.Mutex
}

func newJSONLineWriter(w io.Writer) *jsonLineWriter {
	return &jsonLineWriter{w: w}
}

func (j *jsonLineWriter) writeRecord(record map[string]any) {
	if record == nil {
		return
	}
	if _, ok := record["ts"]; !ok {
		record["ts"] = time.Now().UTC().Format(time.RFC3339Nano)
	}
	b, err := json.Marshal(record)
	if err != nil {
		logWriteFallback("logging: JSON 行序列化失败", err)
		return
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	if _, err := j.w.Write(append(b, '\n')); err != nil {
		logWriteFallback("logging: JSON 行写入失败", err)
	}
}

func argsToMap(args ...any) map[string]any {
	out := make(map[string]any, len(args)/2+1)
	for i := 0; i+1 < len(args); i += 2 {
		key, ok := args[i].(string)
		if !ok {
			continue
		}
		out[key] = args[i+1]
	}
	return out
}

func mergeMaps(base, extra map[string]any) map[string]any {
	out := make(map[string]any, len(base)+len(extra))
	for k, v := range base {
		out[k] = v
	}
	for k, v := range extra {
		out[k] = v
	}
	return out
}

func ctxFieldsMap(ctx context.Context) map[string]any {
	return argsToMap(fieldsFrom(ctx)...)
}
