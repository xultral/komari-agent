package main

import (
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

var forbiddenMonitoringOnlyPaths = []string{
	"terminal",
	"update",
	"server/task.go",
	"server/task_test.go",
	"server/task_exec_test.go",
	"server/task_exec_windows_test.go",
	"server/files.go",
	"server/files_test.go",
	"server/file_stream.go",
	"server/file_stream_test.go",
	"server/files_unix.go",
	"server/files_windows.go",
	"server/files_windows_test.go",
	"server/startup_config.go",
	"server/switch_version.go",
	"server/switch_version_test.go",
	"cmd/warn.go",
	"cmd/warn_windows.go",
	"cmd/warn_linux.go",
	"cmd/warn_linux_test.go",
	"cmd/warning.go",
}

var forbiddenMonitoringOnlyImports = map[string]struct{}{
	"github.com/UserExistsError/conpty":        {},
	"github.com/creack/pty":                    {},
	"github.com/prometheus-community/pro-bing": {},
	"github.com/rhysd/go-github-selfupdate":    {},
	"gopkg.in/toast.v1":                        {},
}

func TestMonitoringOnlyPathsStayRemoved(t *testing.T) {
	root := repositoryRoot(t)
	for _, relativePath := range forbiddenMonitoringOnlyPaths {
		path := filepath.Join(root, relativePath)
		info, err := os.Stat(path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			t.Fatalf("failed to inspect %s: %v", relativePath, err)
		}
		if !info.IsDir() {
			t.Errorf("monitoring-only invariant violated: %s must not exist", relativePath)
			continue
		}
		entries, err := os.ReadDir(path)
		if err != nil {
			t.Fatalf("failed to inspect %s: %v", relativePath, err)
		}
		if len(entries) != 0 {
			t.Errorf("monitoring-only invariant violated: %s must be empty", relativePath)
		}
	}
}

func TestMonitoringOnlyImportsStayRemoved(t *testing.T) {
	root := repositoryRoot(t)

	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			switch entry.Name() {
			case ".git", ".gocache", ".gomodcache", "build", "upstream-latest", "vendor":
				return fs.SkipDir
			default:
				return nil
			}
		}
		if !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			return nil
		}

		relativePath, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		relativePath = filepath.ToSlash(relativePath)

		file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if err != nil {
			t.Errorf("failed to parse %s: %v", relativePath, err)
			return nil
		}
		for _, imported := range file.Imports {
			importPath, err := strconv.Unquote(imported.Path.Value)
			if err != nil {
				t.Errorf("failed to read import in %s: %v", relativePath, err)
				continue
			}
			if _, forbidden := forbiddenMonitoringOnlyImports[importPath]; forbidden {
				t.Errorf("monitoring-only invariant violated: %s imports %s", relativePath, importPath)
			}
			if importPath == "os/exec" && (strings.HasPrefix(relativePath, "server/") || strings.HasPrefix(relativePath, "cmd/")) {
				t.Errorf("monitoring-only invariant violated: %s imports os/exec", relativePath)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("failed to scan repository: %v", err)
	}
}

func TestGoModDoesNotCarryRemovedRemoteControlDependencies(t *testing.T) {
	root := repositoryRoot(t)
	goMod, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		t.Fatalf("failed to read go.mod: %v", err)
	}
	for dependency := range forbiddenMonitoringOnlyImports {
		if strings.Contains(string(goMod), dependency) {
			t.Errorf("monitoring-only invariant violated: go.mod requires %s", dependency)
		}
	}
}

func repositoryRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("failed to locate repository root")
	}
	return filepath.Dir(file)
}
