package counter

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/linfunss/codecounter/internal/ignore"
)

// demoSource 是一段同时包含代码、注释与空行的 Go 源码，
// 用于验证统计结果是否与预期一致。
const demoSource = `// 注释一
package demo

// 注释二
func Add(a, b int) int {
	// 注释三
	return a + b
}

/* 块注释
   第二行 */
func Sub(a, b int) int {
	return a - b
}
`

// TestDetectLanguage 验证常见扩展名的语言识别结果。
func TestDetectLanguage(t *testing.T) {
	cases := map[string]string{
		"main.go":      "Go",
		"app.py":       "Python",
		"index.js":     "JavaScript",
		"index.ts":     "TypeScript",
		"style.css":    "CSS",
		"README.md":    "Markdown",
		"unknown.zzzz": "",
	}

	for name, want := range cases {
		if got := DetectLanguage(name, []byte("")); got != want {
			t.Errorf("DetectLanguage(%q) = %q, 期望 %q", name, got, want)
		}
	}
}

// TestDetectLanguageByShebang 验证无扩展名脚本可以通过 shebang 识别。
func TestDetectLanguageByShebang(t *testing.T) {
	content := []byte("#!/usr/bin/env python3\nprint('hi')\n")
	if got := DetectLanguage("run", content); got != "Python" {
		t.Errorf("DetectLanguage 通过 shebang 得到 %q, 期望 Python", got)
	}
}

// TestCountDirAggregates 验证目录统计的聚合结果。
func TestCountDirAggregates(t *testing.T) {
	dir := t.TempDir()

	if err := os.WriteFile(filepath.Join(dir, "demo.go"), []byte(demoSource), 0o644); err != nil {
		t.Fatalf("写入测试文件失败: %v", err)
	}
	// 冗余目录中的文件不应被统计
	if err := os.MkdirAll(filepath.Join(dir, "node_modules"), 0o755); err != nil {
		t.Fatalf("创建目录失败: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "node_modules", "index.js"), []byte("const a = 1;\n"), 0o644); err != nil {
		t.Fatalf("写入测试文件失败: %v", err)
	}

	matcher, err := ignore.New(ignore.Options{UseDefaults: true})
	if err != nil {
		t.Fatalf("构建 Matcher 失败: %v", err)
	}

	res, err := CountDir(context.Background(), dir, Options{Matcher: matcher})
	if err != nil {
		t.Fatalf("统计失败: %v", err)
	}

	if res.Files != 1 {
		t.Fatalf("统计文件数 = %d, 期望 1", res.Files)
	}
	if len(res.Languages) != 1 {
		t.Fatalf("语言数量 = %d, 期望 1", len(res.Languages))
	}

	stat := res.Languages[0]
	if stat.Language != "Go" {
		t.Errorf("语言 = %q, 期望 Go", stat.Language)
	}
	if stat.Code != 7 {
		t.Errorf("代码行 = %d, 期望 7", stat.Code)
	}
	if stat.Comment != 5 {
		t.Errorf("注释行 = %d, 期望 5", stat.Comment)
	}
	if stat.Blank != 2 {
		t.Errorf("空行 = %d, 期望 2", stat.Blank)
	}
	if stat.Lines != stat.Code+stat.Comment+stat.Blank {
		t.Errorf("总行数 %d 与代码/注释/空行之和不一致", stat.Lines)
	}

	if res.Total.Code != stat.Code {
		t.Errorf("汇总代码行 = %d, 期望 %d", res.Total.Code, stat.Code)
	}
}

// TestCountFiles 验证显式文件列表的统计。
func TestCountFiles(t *testing.T) {
	dir := t.TempDir()
	goFile := filepath.Join(dir, "demo.go")
	if err := os.WriteFile(goFile, []byte(demoSource), 0o644); err != nil {
		t.Fatalf("写入测试文件失败: %v", err)
	}

	res, err := CountFiles(context.Background(), []string{goFile}, Options{})
	if err != nil {
		t.Fatalf("统计失败: %v", err)
	}
	if res.Files != 1 {
		t.Fatalf("统计文件数 = %d, 期望 1", res.Files)
	}
	if res.Root != "" {
		t.Errorf("文件模式下 Root 应为空，实际为 %q", res.Root)
	}
}

// TestCountDirMissing 验证目录不存在时返回错误。
func TestCountDirMissing(t *testing.T) {
	if _, err := CountDir(context.Background(), filepath.Join(t.TempDir(), "missing"), Options{}); err == nil {
		t.Error("统计不存在的目录时应返回错误")
	}
}

// TestSortStats 验证排序规则：代码行降序，其次注释行、语言名。
func TestSortStats(t *testing.T) {
	stats := []LanguageStat{
		{Language: "B", Code: 10, Comment: 1},
		{Language: "A", Code: 10, Comment: 5},
		{Language: "C", Code: 100},
		{Language: "D", Code: 10, Comment: 5},
	}
	SortStats(stats)

	want := []string{"C", "A", "D", "B"}
	for i, name := range want {
		if stats[i].Language != name {
			t.Fatalf("排序结果 = %v, 期望 %v", stats, want)
		}
	}
}
