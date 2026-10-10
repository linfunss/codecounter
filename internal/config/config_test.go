package config

import (
	"os"
	"path/filepath"
	"testing"
)

// TestDefaults 验证默认配置的关键取值。
func TestDefaults(t *testing.T) {
	cfg := Defaults()

	if cfg.DefaultDirMode != DirModeWorkingDir {
		t.Errorf("默认目录策略 = %q, 期望 %q", cfg.DefaultDirMode, DirModeWorkingDir)
	}
	if !cfg.UseDefaultExcludes {
		t.Error("默认应启用内置排除名单")
	}
	if !cfg.UseGitIgnore {
		t.Error("默认应读取 .gitignore")
	}
	if !cfg.ShowLanguageIcons {
		t.Error("默认应显示语言图标")
	}
	if cfg.MaxBars <= 0 {
		t.Errorf("默认条形图数量上限应为正数，实际 %d", cfg.MaxBars)
	}
}

// TestNormalize 验证非法字段会被修正。
func TestNormalize(t *testing.T) {
	cfg := Config{
		DefaultDirMode: DirMode("不存在的模式"),
		MaxBars:        -5,
		CustomExcludes: []string{"  ", "node_modules", "", "  *.min.js  "},
	}
	cfg.Normalize()

	if cfg.DefaultDirMode != DirModeWorkingDir {
		t.Errorf("非法目录策略应回退为工作目录，实际 %q", cfg.DefaultDirMode)
	}
	if cfg.MaxBars != Defaults().MaxBars {
		t.Errorf("负数上限应回退为默认值，实际 %d", cfg.MaxBars)
	}
	if len(cfg.CustomExcludes) != 2 {
		t.Fatalf("自定义规则应剔除空行，实际 %v", cfg.CustomExcludes)
	}
	if cfg.CustomExcludes[0] != "node_modules" || cfg.CustomExcludes[1] != "*.min.js" {
		t.Errorf("自定义规则应保留内容并去除首尾空白，实际 %v", cfg.CustomExcludes)
	}
}

// TestInitialDirFixed 验证固定目录策略。
func TestInitialDirFixed(t *testing.T) {
	dir := t.TempDir()

	cfg := Defaults()
	cfg.DefaultDirMode = DirModeFixed
	cfg.FixedDir = dir
	if got := cfg.InitialDir(); got != dir {
		t.Errorf("InitialDir() = %q, 期望 %q", got, dir)
	}

	// 目录不存在时回退到当前工作目录
	cfg.FixedDir = filepath.Join(dir, "missing")
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("获取工作目录失败: %v", err)
	}
	if got := cfg.InitialDir(); got != wd {
		t.Errorf("目录不存在时应回退到工作目录，实际 %q", got)
	}
}

// TestInitialDirLast 验证上次目录策略。
func TestInitialDirLast(t *testing.T) {
	dir := t.TempDir()

	cfg := Defaults()
	cfg.DefaultDirMode = DirModeLast
	cfg.LastDir = dir
	if got := cfg.InitialDir(); got != dir {
		t.Errorf("InitialDir() = %q, 期望 %q", got, dir)
	}
}

// TestNewMatcher 验证配置可以正确转换为排除规则匹配器。
func TestNewMatcher(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte("secret/\n"), 0o644); err != nil {
		t.Fatalf("写入 .gitignore 失败: %v", err)
	}

	cfg := Defaults()
	cfg.CustomExcludes = []string{"*.min.js"}

	matcher, err := cfg.NewMatcher(dir)
	if err != nil {
		t.Fatalf("构建 Matcher 失败: %v", err)
	}

	if !matcher.Skip("node_modules", true) {
		t.Error("内置名单应跳过 node_modules")
	}
	if !matcher.Skip("app.min.js", false) {
		t.Error("自定义规则应跳过 *.min.js")
	}
	if !matcher.Skip("secret", true) {
		t.Error(".gitignore 规则应生效")
	}
	if matcher.Skip("main.go", false) {
		t.Error("main.go 不应被跳过")
	}
}

// TestNewMatcherDisablesRules 验证关闭开关后规则不再生效。
func TestNewMatcherDisablesRules(t *testing.T) {
	cfg := Defaults()
	cfg.UseDefaultExcludes = false
	cfg.UseGitIgnore = false

	matcher, err := cfg.NewMatcher(t.TempDir())
	if err != nil {
		t.Fatalf("构建 Matcher 失败: %v", err)
	}
	if matcher.Skip("node_modules", true) {
		t.Error("关闭内置名单后不应跳过 node_modules")
	}
}
