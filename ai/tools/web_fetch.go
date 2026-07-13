package tools

import (
	"context"
	"fmt"
	"io"
	"matrix/ai/util"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// NewWebFetchTool 创建「按 URL 拉取页面内容」的工具。
//
// 职责边界：已知 URL 时的 HTTP GET 抓取与 HTML→纯文本转换。
// 与 [NewWebSearchTool]（关键词检索、返回多条搜索结果）无关，勿混用。
//
// WebFetchTool：
//   - HTTP GET 请求（带 30s 超时）
//   - HTML 自动转换为纯文本（去除标签和脚本）
//   - 内容长度上限 200KB（防止 context 爆炸）
//   - 并发安全（只读操作）
//
// 注意：仅适用于公开 URL，不支持需要认证的服务
// （对应源码中 "WebFetch WILL FAIL for authenticated or private URLs" 警告）。
func NewWebFetchTool() *Tool {
	return &Tool{
		Name:        "web_fetch",
		Description: "对已知 URL 执行 HTTP GET，将响应体转为可读文本（HTML 会去标签）。与「关键词网络搜索」不同：此处必须提供完整 URL，不发起搜索引擎查询。",
		Parameters: JSONSchema{
			Type: "object",
			Properties: map[string]PropSchema{
				"url": {
					Type:        "string",
					Description: "要打开的完整 URL（必须是公开可访问的）",
				},
				"prompt": {
					Type:        "string",
					Description: "可选：对抓取内容的筛选提示，当前直接返回全部内容（未来可接入 LLM 摘要）",
				},
			},
			Required: []string{"url"},
		},
		IsConcurrencySafe: true,
		StatusLabel: func(args map[string]any) string {
			rawURL, ok := getString(args, "url")
			if !ok {
				return ""
			}
			return fmt.Sprintf("获取 %s …", rawURL)
		},
		Execute: execWebFetch,
	}
}

const (
	webFetchMaxBodyBytes = 200 * 1024 // 200 KB 正文上限
	webFetchTimeout      = 30 * time.Second
)

// execWebFetch 是 web_fetch 工具的执行逻辑。
func execWebFetch(ctx context.Context, args map[string]any) (string, error) {
	rawURL, _ := getString(args, "url")
	if _, err := url.ParseRequestURI(rawURL); err != nil {
		return "", fmt.Errorf("web_fetch: 无效的 URL %q: %w", rawURL, err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return "", fmt.Errorf("web_fetch: 创建请求失败: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; matrix-agent/1.0)")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,text/plain;q=0.9")
	client := &http.Client{Timeout: webFetchTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("web_fetch: 请求失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("web_fetch: 服务器返回 %d %s", resp.StatusCode, resp.Status)
	}
	limited := io.LimitReader(resp.Body, webFetchMaxBodyBytes+1)
	var sb strings.Builder
	w := util.StreamWriter(ctx)
	buf := make([]byte, readFileChunkSize)
	truncated := false
	for {
		n, readErr := limited.Read(buf)
		if n > 0 {
			sb.WriteString(string(buf[:n]))
			_, _ = w.Write(buf[:n])
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return "", fmt.Errorf("web_fetch: 读取响应失败: %w", readErr)
		}
	}
	body := []byte(sb.String())
	if len(body) > webFetchMaxBodyBytes {
		truncated = true
		body = body[:webFetchMaxBodyBytes]
	}
	contentType := resp.Header.Get("Content-Type")
	var text string
	if strings.Contains(contentType, "text/html") {
		text = htmlToText(string(body))
	} else {
		text = string(body)
	}
	if truncated {
		text += "\n\n[内容已截断，仅显示前 200KB]"
	}
	return fmt.Sprintf("[%d %s] %s\n\n%s", resp.StatusCode, resp.Status, rawURL, text), nil
}

// htmlToText 将 HTML 转换为纯文本。
func htmlToText(html string) string {
	html = reScript.ReplaceAllString(html, "")
	html = reStyle.ReplaceAllString(html, "")
	html = reBlock.ReplaceAllString(html, "\n")
	html = reTag.ReplaceAllString(html, "")
	html = decodeHTMLEntities(html)
	html = reMultiSpace.ReplaceAllString(html, " ")
	html = reMultiNewline.ReplaceAllString(html, "\n\n")
	return strings.TrimSpace(html)
}

var (
	reScript       = regexp.MustCompile(`(?is)<script[^>]*>.*?</script>`)
	reStyle        = regexp.MustCompile(`(?is)<style[^>]*>.*?</style>`)
	reBlock        = regexp.MustCompile(`(?i)<(?:br|p|div|h[1-6]|li|tr|td|th|blockquote|pre|hr)[^>]*>`)
	reTag          = regexp.MustCompile(`<[^>]+>`)
	reMultiSpace   = regexp.MustCompile(`[ \t]+`)
	reMultiNewline = regexp.MustCompile(`\n{3,}`)
)

// decodeHTMLEntities 解码常见的 HTML 实体。
func decodeHTMLEntities(s string) string {
	replacer := strings.NewReplacer(
		"&amp;", "&",
		"&lt;", "<",
		"&gt;", ">",
		"&quot;", "\"",
		"&#39;", "'",
		"&apos;", "'",
		"&nbsp;", " ",
		"&mdash;", "—",
		"&ndash;", "–",
		"&hellip;", "…",
		"&copy;", "©",
		"&reg;", "®",
	)
	return replacer.Replace(s)
}
