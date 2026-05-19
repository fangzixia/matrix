package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os/exec"
	"sync"
	"sync/atomic"
	"time"
)

// Client MCP 客户端
type Client struct {
	cmd           *exec.Cmd
	stdin         io.WriteCloser
	stdout        io.ReadCloser
	stderr        io.ReadCloser
	scanner       *bufio.Scanner
	requestID     atomic.Int64
	pendingCalls  sync.Map // map[interface{}]chan *JSONRPCResponse
	initialized   bool
	serverInfo    *ServerInfo
	capabilities  *ServerCapabilities
	mu            sync.RWMutex
	ctx           context.Context
	cancel        context.CancelFunc
	closed        bool
	notifications chan *JSONRPCNotification
}

// NewClient 创建 MCP 客户端
func NewClient(command string, args []string, env []string) (*Client, error) {
	ctx, cancel := context.WithCancel(context.Background())

	cmd := exec.CommandContext(ctx, command, args...)
	cmd.Env = append(cmd.Env, env...)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("create stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("create stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("create stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		cancel()
		return nil, fmt.Errorf("start command: %w", err)
	}

	client := &Client{
		cmd:           cmd,
		stdin:         stdin,
		stdout:        stdout,
		stderr:        stderr,
		scanner:       bufio.NewScanner(stdout),
		ctx:           ctx,
		cancel:        cancel,
		notifications: make(chan *JSONRPCNotification, 100),
	}

	// 启动消息读取协程
	go client.readLoop()

	// 启动 stderr 日志协程
	go client.logStderr()

	return client, nil
}

// Initialize 初始化 MCP 连接
func (c *Client) Initialize() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.initialized {
		return nil
	}

	req := InitializeRequest{
		ProtocolVersion: "2024-11-05",
		Capabilities: Capabilities{
			Roots: &RootsCapability{
				ListChanged: true,
			},
			Sampling: &SamplingCapability{},
		},
		ClientInfo: ClientInfo{
			Name:    "matrix",
			Version: "1.0.0",
		},
	}

	var result InitializeResult
	if err := c.call("initialize", req, &result); err != nil {
		return fmt.Errorf("initialize: %w", err)
	}

	c.serverInfo = &result.ServerInfo
	c.capabilities = &result.Capabilities
	c.initialized = true

	// 发送 initialized 通知
	if err := c.notify("notifications/initialized", nil); err != nil {
		return fmt.Errorf("send initialized notification: %w", err)
	}

	log.Printf("MCP client initialized: server=%s version=%s", result.ServerInfo.Name, result.ServerInfo.Version)

	return nil
}

// ListTools 列出所有工具
func (c *Client) ListTools() ([]Tool, error) {
	if !c.initialized {
		if err := c.Initialize(); err != nil {
			return nil, err
		}
	}

	var result ListToolsResult
	if err := c.call("tools/list", nil, &result); err != nil {
		return nil, fmt.Errorf("list tools: %w", err)
	}

	return result.Tools, nil
}

// CallTool 调用工具
func (c *Client) CallTool(name string, arguments map[string]interface{}) (*CallToolResult, error) {
	if !c.initialized {
		if err := c.Initialize(); err != nil {
			return nil, err
		}
	}

	req := CallToolRequest{
		Name:      name,
		Arguments: arguments,
	}

	var result CallToolResult
	if err := c.call("tools/call", req, &result); err != nil {
		return nil, fmt.Errorf("call tool: %w", err)
	}

	return &result, nil
}

// ListResources 列出所有资源
func (c *Client) ListResources() ([]Resource, error) {
	if !c.initialized {
		if err := c.Initialize(); err != nil {
			return nil, err
		}
	}

	var result ListResourcesResult
	if err := c.call("resources/list", nil, &result); err != nil {
		return nil, fmt.Errorf("list resources: %w", err)
	}

	return result.Resources, nil
}

// ReadResource 读取资源
func (c *Client) ReadResource(uri string) (*ReadResourceResult, error) {
	if !c.initialized {
		if err := c.Initialize(); err != nil {
			return nil, err
		}
	}

	req := ReadResourceRequest{URI: uri}

	var result ReadResourceResult
	if err := c.call("resources/read", req, &result); err != nil {
		return nil, fmt.Errorf("read resource: %w", err)
	}

	return &result, nil
}

// ListPrompts 列出所有提示
func (c *Client) ListPrompts() ([]Prompt, error) {
	if !c.initialized {
		if err := c.Initialize(); err != nil {
			return nil, err
		}
	}

	var result ListPromptsResult
	if err := c.call("prompts/list", nil, &result); err != nil {
		return nil, fmt.Errorf("list prompts: %w", err)
	}

	return result.Prompts, nil
}

// GetServerInfo 获取服务器信息
func (c *Client) GetServerInfo() *ServerInfo {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.serverInfo
}

// GetCapabilities 获取服务器能力
func (c *Client) GetCapabilities() *ServerCapabilities {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.capabilities
}

// Close 关闭客户端
func (c *Client) Close() error {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil
	}
	c.closed = true
	c.mu.Unlock()

	// 取消上下文
	c.cancel()

	// 关闭管道
	if c.stdin != nil {
		c.stdin.Close()
	}

	// 等待进程结束（带超时）
	done := make(chan error, 1)
	go func() {
		done <- c.cmd.Wait()
	}()

	select {
	case <-time.After(5 * time.Second):
		// 超时，强制杀死进程
		if c.cmd.Process != nil {
			c.cmd.Process.Kill()
		}
		return fmt.Errorf("timeout waiting for process to exit")
	case err := <-done:
		return err
	}
}

// call 发送 JSON-RPC 请求并等待响应
func (c *Client) call(method string, params interface{}, result interface{}) error {
	c.mu.RLock()
	if c.closed {
		c.mu.RUnlock()
		return fmt.Errorf("client is closed")
	}
	c.mu.RUnlock()

	id := c.requestID.Add(1)
	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  params,
	}

	// 创建响应通道
	respChan := make(chan *JSONRPCResponse, 1)
	c.pendingCalls.Store(id, respChan)
	defer c.pendingCalls.Delete(id)

	// 发送请求
	data, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	if _, err := c.stdin.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("write request: %w", err)
	}

	// 等待响应（带超时）
	select {
	case resp := <-respChan:
		if resp.Error != nil {
			return fmt.Errorf("rpc error %d: %s", resp.Error.Code, resp.Error.Message)
		}

		if result != nil && resp.Result != nil {
			// 将 result 重新编码为 JSON，然后解码到目标类型
			resultData, err := json.Marshal(resp.Result)
			if err != nil {
				return fmt.Errorf("marshal result: %w", err)
			}
			if err := json.Unmarshal(resultData, result); err != nil {
				return fmt.Errorf("unmarshal result: %w", err)
			}
		}

		return nil
	case <-time.After(30 * time.Second):
		return fmt.Errorf("request timeout")
	case <-c.ctx.Done():
		return fmt.Errorf("client closed")
	}
}

// notify 发送 JSON-RPC 通知（无需响应）
func (c *Client) notify(method string, params interface{}) error {
	c.mu.RLock()
	if c.closed {
		c.mu.RUnlock()
		return fmt.Errorf("client is closed")
	}
	c.mu.RUnlock()

	notif := JSONRPCNotification{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
	}

	data, err := json.Marshal(notif)
	if err != nil {
		return fmt.Errorf("marshal notification: %w", err)
	}

	if _, err := c.stdin.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("write notification: %w", err)
	}

	return nil
}

// readLoop 读取服务器消息
func (c *Client) readLoop() {
	for c.scanner.Scan() {
		line := c.scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		// 尝试解析为响应
		var resp JSONRPCResponse
		if err := json.Unmarshal(line, &resp); err == nil && resp.ID != nil {
			// 这是一个响应
			if ch, ok := c.pendingCalls.Load(resp.ID); ok {
				select {
				case ch.(chan *JSONRPCResponse) <- &resp:
				default:
					log.Printf("Warning: response channel full for ID %v", resp.ID)
				}
			} else {
				log.Printf("Warning: received response for unknown ID %v", resp.ID)
			}
			continue
		}

		// 尝试解析为通知
		var notif JSONRPCNotification
		if err := json.Unmarshal(line, &notif); err == nil && notif.Method != "" {
			// 这是一个通知
			select {
			case c.notifications <- &notif:
			default:
				log.Printf("Warning: notification channel full, dropping notification: %s", notif.Method)
			}
			continue
		}

		log.Printf("Warning: received unknown message: %s", string(line))
	}

	if err := c.scanner.Err(); err != nil {
		log.Printf("Error reading from stdout: %v", err)
	}
}

// logStderr 记录 stderr 输出
func (c *Client) logStderr() {
	scanner := bufio.NewScanner(c.stderr)
	for scanner.Scan() {
		log.Printf("[MCP stderr] %s", scanner.Text())
	}
}
