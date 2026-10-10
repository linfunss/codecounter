package ignore

import (
	"os"
	"path/filepath"
	"testing"
)

// TestGitIgnoreSelfRepository 用本仓库真实的 .gitignore 验证常见目录会被跳过。
func TestGitIgnoreSelfRepository(t *testing.T) {
	path := filepath.Join("..", "..", ".gitignore")
	if _, err := os.Stat(path); err != nil {
		t.Skip("仓库 .gitignore 不存在，跳过")
	}

	m, err := New(Options{UseDefaults: true, GitIgnorePath: path})
	if err != nil {
		t.Fatalf("构建 Matcher 失败: %v", err)
	}

	cases := []struct {
		rel   string
		isDir bool
	}{
		{"dist", true},
		{"dist/config.yaml", false},
		{"node_modules", true},
		{".git", true},
		{"codecounter.exe", false},
	}
	for _, c := range cases {
		if !m.Skip(c.rel, c.isDir) {
			t.Errorf("%s 应被跳过", c.rel)
		}
	}

	if m.Skip("internal/ui/chart.go", false) {
		t.Error("源码文件不应被跳过")
	}
}
