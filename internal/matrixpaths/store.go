package matrixpaths

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	dirWorkspaces = "workspaces"
	metaFileName  = "meta.json"
)

// WorkspaceMeta 记录工作区与应用存储目录的映射。
type WorkspaceMeta struct {
	WorkspaceID   string `json:"workspace_id"`
	WorkspacePath string `json:"workspace_path"`
	LastOpenedAt  string `json:"last_opened_at,omitempty"`
}

var dataRootOverride string

// SetDataRootForTest 将应用数据根目录重定向到测试目录。
func SetDataRootForTest(root string) {
	dataRootOverride = root
}

// AppDataDir 返回 Matrix 应用数据根目录（如 %APPDATA%/matrix）。
func AppDataDir() (string, error) {
	if dataRootOverride != "" {
		return dataRootOverride, nil
	}
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(base, AppName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, nil
}

// NormalizeWorkspacePath 规范化工作区绝对路径。
func NormalizeWorkspacePath(workspaceRoot string) string {
	if workspaceRoot == "" {
		return ""
	}
	abs, err := filepath.Abs(workspaceRoot)
	if err != nil {
		return filepath.Clean(workspaceRoot)
	}
	return filepath.Clean(abs)
}

// WorkspaceID 根据工作区绝对路径生成稳定 ID（16 位 hex）。
func WorkspaceID(workspaceRoot string) string {
	norm := NormalizeWorkspacePath(workspaceRoot)
	if norm == "" {
		return ""
	}
	key := strings.ToLower(filepath.ToSlash(norm))
	sum := sha256.Sum256([]byte(key))
	return hex.EncodeToString(sum[:8])
}

func workspaceStoreRoot(workspaceRoot string) string {
	id := WorkspaceID(workspaceRoot)
	if id == "" {
		return ""
	}
	appDir, err := AppDataDir()
	if err != nil {
		return ""
	}
	return filepath.Join(appDir, dirWorkspaces, id)
}

func joinWorkspaceStore(workspaceRoot string, elems ...string) string {
	root := workspaceStoreRoot(workspaceRoot)
	if root == "" {
		return ""
	}
	parts := append([]string{root}, elems...)
	return filepath.Join(parts...)
}

func metaPath(workspaceRoot string) string {
	root := workspaceStoreRoot(workspaceRoot)
	if root == "" {
		return ""
	}
	return filepath.Join(root, metaFileName)
}

// EnsureWorkspaceStore 确保应用数据目录存在并更新 meta。
func EnsureWorkspaceStore(workspaceRoot string) error {
	abs := NormalizeWorkspacePath(workspaceRoot)
	if abs == "" {
		return nil
	}
	root := workspaceStoreRoot(abs)
	if root == "" {
		return fmt.Errorf("workspace store root unavailable")
	}
	for _, sub := range []string{DirSessions, DirChatTranscripts, DirSubagents, DirExports} {
		if err := os.MkdirAll(filepath.Join(root, sub), 0o755); err != nil {
			return err
		}
	}

	meta, _ := readMeta(abs)
	if meta == nil {
		meta = &WorkspaceMeta{}
	}
	meta.WorkspaceID = WorkspaceID(abs)
	meta.WorkspacePath = abs
	meta.LastOpenedAt = time.Now().UTC().Format(time.RFC3339)
	return writeMeta(abs, meta)
}

func readMeta(workspaceRoot string) (*WorkspaceMeta, error) {
	path := metaPath(workspaceRoot)
	if path == "" {
		return nil, fmt.Errorf("meta path unavailable")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var meta WorkspaceMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, err
	}
	return &meta, nil
}

func writeMeta(workspaceRoot string, meta *WorkspaceMeta) error {
	path := metaPath(workspaceRoot)
	if path == "" || meta == nil {
		return fmt.Errorf("meta path unavailable")
	}
	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
