package ignore

import (
	"os"
	"path/filepath"
	"testing"
)

// TestSkipDefaultRules 验证内置名单能命中常见的冗余目录。
func TestSkipDefaultRules(t *testing.T) {
	m, err := New(Options{UseDefaults: true})
	if err != nil {
		t.Fatalf("构建 Matcher 失败: %v", err)
	}

	cases := []struct {
		rel   string
		isDir bool
		want  bool
	}{
		{"node_modules", true, true},
		{"a/b/node_modules", true, true},
		{".idea", true, true},
		{"src", true, false},
		{"src/main.go", false, false},
		{"README.md", false, false},
		{".gitignore", false, true},
		{"go.sum", false, true},
		{"package-lock.json", false, true},
		{"dist/bundle.js", false, false},
	}

	for _, c := range cases {
		if got := m.Skip(c.rel, c.isDir); got != c.want {
			t.Errorf("Skip(%q, isDir=%v) = %v, 期望 %v", c.rel, c.isDir, got, c.want)
		}
	}
}

// TestSkipWithoutDefaults 验证关闭内置名单后不再过滤。
func TestSkipWithoutDefaults(t *testing.T) {
	m, err := New(Options{UseDefaults: false})
	if err != nil {
		t.Fatalf("构建 Matcher 失败: %v", err)
	}
	if m.Skip("node_modules", true) {
		t.Error("未启用内置名单时不应跳过 node_modules")
	}
}

// TestCustomRules 验证自定义规则、glob 与取反。
func TestCustomRules(t *testing.T) {
	m, err := New(Options{
		UseDefaults: false,
		Custom: []string{
			"",            // 空行应被忽略
			"# 这是注释",      // 注释应被忽略
			"*.min.js",    // 按文件名 glob 匹配
			"build",       // 目录名匹配
			"docs/",       // 仅匹配目录
			"!build/keep", // 取反示例
		},
	})
	if err != nil {
		t.Fatalf("构建 Matcher 失败: %v", err)
	}

	cases := []struct {
		rel   string
		isDir bool
		want  bool
	}{
		{"app.min.js", false, true},
		{"app.js", false, false},
		{"build", true, true},
		{"docs", true, true},
		{"docs", false, false},
	}
	for _, c := range cases {
		if got := m.Skip(c.rel, c.isDir); got != c.want {
			t.Errorf("Skip(%q, isDir=%v) = %v, 期望 %v", c.rel, c.isDir, got, c.want)
		}
	}
}

// TestGitIgnoreRule 验证可以读取 .gitignore 中的规则。
func TestGitIgnoreRule(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".gitignore")
	if err := os.WriteFile(path, []byte("secret/\n*.log\n"), 0o644); err != nil {
		t.Fatalf("写入 .gitignore 失败: %v", err)
	}

	m, err := New(Options{GitIgnorePath: path})
	if err != nil {
		t.Fatalf("构建 Matcher 失败: %v", err)
	}

	if !m.Skip("secret", true) {
		t.Error("secret 目录应被 .gitignore 跳过")
	}
	if !m.Skip("build/app.log", false) {
		t.Error("*.log 应被 .gitignore 跳过")
	}
	if m.Skip("main.go", false) {
		t.Error("main.go 不应被 .gitignore 跳过")
	}
}

// TestNormalizeRel 验证相对路径归一化。
func TestNormalizeRel(t *testing.T) {
	cases := map[string]string{
		"./src\\main.go": "src/main.go",
		"/a/b/":          "a/b",
		".":              "",
		"":               "",
	}
	for in, want := range cases {
		if got := normalizeRel(in); got != want {
			t.Errorf("normalizeRel(%q) = %q, 期望 %q", in, got, want)
		}
	}
}
