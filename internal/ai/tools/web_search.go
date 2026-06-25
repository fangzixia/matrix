package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// NewWebSearchTool 创建「关键词网络搜索」工具。
//
// 职责边界：根据查询词调用搜索引擎 API，返回多条 {标题, URL, 摘要}。
// 与 [NewWebFetchTool]（对单个已知 URL 做 GET 拉取）无关，勿混用。
//
// WebSearchTool：通用 HTTP 搜索接口。
//   - 使用 Brave Search API（设置 BRAVE_SEARCH_API_KEY 环境变量）
//   - 未配置 API Key 时返回明确的配置提示
//
// Brave Search API 申请：https://api.search.brave.com/register
func NewWebSearchTool() *Tool {
	return &Tool{
		Name:        "web_search",
		Description: "根据关键词检索公开网页，返回多条搜索结果（标题、链接、摘要）。需配置 BRAVE_SEARCH_API_KEY；不能替代 web_fetch——若已知文档 URL，应优先用 web_fetch 直接打开。",
		Parameters: JSONSchema{
			Type: "object",
			Properties: map[string]PropSchema{
				"query": {
					Type:        "string",
					Description: "搜索查询词",
				},
				"count": {
					Type:        "integer",
					Description: "返回结果数（默认 10，最大 20）",
				},
			},
			Required: []string{"query"},
		},
		IsConcurrencySafe: true,
		Execute:           execWebSearch,
	}
}

// braveSearchResult 是 Brave Search API 响应的部分结构。
type braveSearchResult struct {
	Web struct {
		Results []struct {
			Title       string `json:"title"`
			URL         string `json:"url"`
			Description string `json:"description"`
		} `json:"results"`
	} `json:"web"`
}

// execWebSearch 是 web_search 工具的执行逻辑。
func execWebSearch(ctx context.Context, args map[string]any) (string, error) {
	apiKey := os.Getenv("BRAVE_SEARCH_API_KEY")
	if apiKey == "" {
		return "", fmt.Errorf(
			"web_search: 未配置搜索 API Key。\n" +
				"请设置环境变量 BRAVE_SEARCH_API_KEY（Brave Search 免费层每月 2000 次）。\n" +
				"申请地址：https://api.search.brave.com/register")
	}
	query, _ := getString(args, "query")
	EmitStatus(ctx, fmt.Sprintf("搜索: %s …", query))
	count := 10
	if v, ok := args["count"].(float64); ok && v > 0 {
		count = minInt(int(v), 20)
	}
	apiURL := fmt.Sprintf(
		"https://api.search.brave.com/res/v1/web/search?q=%s&count=%d",
		url.QueryEscape(query), count,
	)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return "", fmt.Errorf("web_search: 创建请求失败: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Subscription-Token", apiKey)
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("web_search: 请求失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return "", fmt.Errorf("web_search: API 返回 %d: %s", resp.StatusCode, body)
	}
	var result braveSearchResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("web_search: 解析响应失败: %w", err)
	}
	if len(result.Web.Results) == 0 {
		return fmt.Sprintf("未找到 %q 的搜索结果", query), nil
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("搜索 %q 的结果：\n\n", query))
	for i, r := range result.Web.Results {
		sb.WriteString(fmt.Sprintf("%d. **%s**\n   URL: %s\n", i+1, r.Title, r.URL))
		if r.Description != "" {
			sb.WriteString(fmt.Sprintf("   %s\n", r.Description))
		}
		sb.WriteByte('\n')
	}
	text := sb.String()
	EmitChunks(ctx, text, defaultEmitChunkSize)
	return text, nil
}
