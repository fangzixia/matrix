// Command buildfrontend 构建 frontend/dist，供 go:generate 与 CI 使用。
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func main() {
	if os.Getenv("SKIP_FRONTEND") != "" {
		return
	}

	root, err := findModuleRoot()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	frontend := filepath.Join(root, "frontend")
	if _, err := os.Stat(filepath.Join(frontend, "package.json")); err != nil {
		fmt.Fprintf(os.Stderr, "frontend/package.json not found under %s\n", root)
		os.Exit(1)
	}

	if _, err := os.Stat(filepath.Join(frontend, "node_modules")); err != nil {
		if err := runDir(frontend, "npm", "install"); err != nil {
			os.Exit(1)
		}
	}

	if err := runDir(frontend, "npm", "run", "build"); err != nil {
		os.Exit(1)
	}
}

func findModuleRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("go.mod not found")
		}
		dir = parent
	}
}

func runDir(dir, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s %v in %s: %w", name, args, dir, err)
	}
	return nil
}
