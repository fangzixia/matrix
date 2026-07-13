package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

type loggingTransport struct {
	base   http.RoundTripper
	client *Client
}

func newLoggingTransport(base http.RoundTripper, client *Client) *loggingTransport {
	if base == nil {
		base = http.DefaultTransport
	}
	return &loggingTransport{base: base, client: client}
}

func (t *loggingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	start := time.Now()
	ctx := req.Context()

	var reqBody []byte
	if req.Body != nil {
		body, err := io.ReadAll(req.Body)
		if err != nil {
			return nil, err
		}
		reqBody = body
		req.Body = io.NopCloser(bytes.NewReader(body))
		// 与标准 Transport 一致：重定向（307/308）时需能重读 POST body。
		req.GetBody = func() (io.ReadCloser, error) {
			return io.NopCloser(bytes.NewReader(body)), nil
		}
	}

	meta := t.httpMeta(req, reqBody)
	logHTTPRequest(ctx, meta, string(reqBody))

	rec := &responseRecorder{ctx: ctx, meta: meta, start: start}

	resp, err := t.base.RoundTrip(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode == http.StatusOK {
		var buf bytes.Buffer
		resp.Body = &loggingReadCloser{
			Reader:     io.TeeReader(resp.Body, &buf),
			closer:     resp.Body,
			recorder:   rec,
			statusCode: resp.StatusCode,
			body:       &buf,
		}
		return resp, nil
	}

	respBody, readErr := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if readErr != nil {
		respBody = append(respBody, []byte(readErr.Error())...)
	}
	rec.log(resp.StatusCode, string(respBody))
	resp.Body = io.NopCloser(bytes.NewReader(respBody))
	return resp, nil
}

func (t *loggingTransport) httpMeta(req *http.Request, reqBody []byte) HTTPMeta {
	meta := HTTPMeta{
		URL:     req.URL.String(),
		BaseURL: strings.TrimRight(t.client.BaseURL, "/"),
		Model:   parseModelFromBody(reqBody),
	}
	if t.client.ModelName != "" {
		meta.ModelName = t.client.ModelName
	}
	return meta
}

func parseModelFromBody(body []byte) string {
	if len(body) == 0 {
		return ""
	}
	var partial struct {
		Model string `json:"model"`
	}
	if err := json.Unmarshal(body, &partial); err != nil {
		return ""
	}
	return partial.Model
}

// responseRecorder 保证每次 HTTP 往返只写一条 llm.response。
type responseRecorder struct {
	once  sync.Once
	ctx   context.Context
	meta  HTTPMeta
	start time.Time
}

func (r *responseRecorder) log(statusCode int, body string) {
	r.once.Do(func() {
		logHTTPResponse(r.ctx, r.meta, statusCode, body, time.Since(r.start))
	})
}

type loggingReadCloser struct {
	io.Reader
	closer     io.Closer
	recorder   *responseRecorder
	statusCode int
	body       *bytes.Buffer
}

func (r *loggingReadCloser) Close() error {
	err := r.closer.Close()
	// 在底层连接关闭后记录已读到的全部字节（流式 200 为一条完整 llm.response）。
	r.recorder.log(r.statusCode, r.body.String())
	return err
}
