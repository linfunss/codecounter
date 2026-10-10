package scan

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/linfunss/codecounter/internal/ignore"
)

// newTestTree 构造一棵包含冗余目录的测试目录树。
func newTestTree(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	files := []string{
		filepath.Join("src", "main.go"),
		filepath.Join("node_modules", "pkg", "index.js"),
		filepath.Join(".idea", "workspace.xml"),
		"README.md",
	}
	for _, rel := range files {
		full := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("创建目录失败: %v", err)
		}
		if err := os.WriteFile(full, []byte("package main\n"), 0o644); err != nil {
			t.Fatalf("写入文件失败: %v", err)
		}
	}
	return root
}

// TestWalkSkipsIgnoredDirs 验证被排除目录不会进入统计队列。
func TestWalkSkipsIgnoredDirs(t *testing.T) {
	root := newTestTree(t)

	matcher, err := ignore.New(ignore.Options{UseDefaults: true})
	if err != nil {
		t.Fatalf("构建 Matcher 失败: %v", err)
	}

	var visited []string
	stats, err := Walk(context.Background(), Options{Root: root, Matcher: matcher}, func(e Entry) error {
		visited = append(visited, e.RelPath)
		return nil
	})
	if err != nil {
		t.Fatalf("遍历失败: %v", err)
	}

	sort.Strings(visited)
	want := []string{"README.md", "src/main.go"}
	if len(visited) != len(want) {
		t.Fatalf("遍历结果 = %v, 期望 %v", visited, want)
	}
	for i := range want {
		if visited[i] != want[i] {
			t.Fatalf("遍历结果 = %v, 期望 %v", visited, want)
		}
	}
	if stats.Files != len(want) {
		t.Errorf("stats.Files = %d, 期望 %d", stats.Files, len(want))
	}
	if stats.Skipped == 0 {
		t.Error("应当记录被跳过的条目")
	}
}

// TestWalkMaxFileSize 验证超过大小上限的文件会被跳过。
func TestWalkMaxFileSize(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "big.go"), make([]byte, 2048), 0o644); err != nil {
		t.Fatalf("写入文件失败: %v", err)
	}

	stats, err := Walk(context.Background(), Options{Root: dir, MaxFileSize: 1024}, nil)
	if err != nil {
		t.Fatalf("遍历失败: %v", err)
	}
	if stats.Files != 0 {
		t.Errorf("stats.Files = %d, 期望 0", stats.Files)
	}
	if stats.Skipped != 1 {
		t.Errorf("stats.Skipped = %d, 期望 1", stats.Skipped)
	}
}

// TestCollectFiles 验证显式文件列表的整理逻辑。
func TestCollectFiles(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "main.go")
	if err := os.WriteFile(file, []byte("package main\n"), 0o644); err != nil {
		t.Fatalf("写入文件失败: %v", err)
	}

	entries, stats := CollectFiles([]string{file, filepath.Join(dir, "missing.go"), dir}, 0)
	if len(entries) != 1 {
		t.Fatalf("收集到 %d 个文件，期望 1 个", len(entries))
	}
	if entries[0].RelPath != "main.go" {
		t.Errorf("RelPath = %q, 期望 main.go", entries[0].RelPath)
	}
	if stats.Skipped != 2 {
		t.Errorf("stats.Skipped = %d, 期望 2", stats.Skipped)
	}
}

// TestWalkContextCanceled 验证上下文取消能中断遍历。
func TestWalkContextCanceled(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"a.go", "b.go", "c.go"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("package main\n"), 0o644); err != nil {
			t.Fatalf("写入文件失败: %v", err)
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := Walk(ctx, Options{Root: dir}, nil); err == nil {
		t.Error("上下文已取消时应返回错误")
	}
}
