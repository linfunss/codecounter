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

// TestDirOverride 验证可以通过环境变量覆盖配置目录。
func TestDirOverride(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(EnvConfigDir, dir)

	got, err := Dir()
	if err != nil {
		t.Fatalf("Dir() 返回错误: %v", err)
	}
	if got != dir {
		t.Errorf("Dir() = %q, 期望 %q", got, dir)
	}

	path, err := Path()
	if err != nil {
		t.Fatalf("Path() 返回错误: %v", err)
	}
	if want := filepath.Join(dir, "config.json"); path != want {
		t.Errorf("Path() = %q, 期望 %q", path, want)
	}
}

// TestSaveLoadRoundTrip 验证配置可以完整保存并读回。
func TestSaveLoadRoundTrip(t *testing.T) {
	t.Setenv(EnvConfigDir, t.TempDir())

	want := Defaults()
	want.DefaultDirMode = DirModeFixed
	want.FixedDir = "D:\\示例目录"
	want.UseDefaultExcludes = false
	want.CustomExcludes = []string{"*.min.js", "docs/"}
	want.ShowLanguageIcons = false
	want.MaxBars = 30

	if err := want.Save(); err != nil {
		t.Fatalf("Save() 返回错误: %v", err)
	}

	got, err := Load()
	if err != nil {
		t.Fatalf("Load() 返回错误: %v", err)
	}

	if got.DefaultDirMode != want.DefaultDirMode || got.FixedDir != want.FixedDir {
		t.Errorf("目录策略未正确保存: %+v", got)
	}
	if got.UseDefaultExcludes != want.UseDefaultExcludes || got.UseGitIgnore != want.UseGitIgnore {
		t.Errorf("排除开关未正确保存: %+v", got)
	}
	if len(got.CustomExcludes) != 2 || got.CustomExcludes[0] != "*.min.js" {
		t.Errorf("自定义规则未正确保存: %v", got.CustomExcludes)
	}
	if got.ShowLanguageIcons != want.ShowLanguageIcons || got.MaxBars != want.MaxBars {
		t.Errorf("显示选项未正确保存: %+v", got)
	}
}

// TestNormalizeScale 验证缩放系数会被限制在合理区间。
func TestNormalizeScale(t *testing.T) {
	cases := []struct {
		in   float64
		want float64
	}{
		{0, Defaults().Scale},
		{0.1, Defaults().Scale},
		{0.8, 0.8},
		{1.25, 1.25},
		{1.6, 1.6},
		{5, Defaults().Scale},
	}

	for _, c := range cases {
		cfg := Defaults()
		cfg.Scale = c.in
		cfg.Normalize()
		if cfg.Scale != c.want {
			t.Errorf("Scale=%v 归一化后 = %v, 期望 %v", c.in, cfg.Scale, c.want)
		}
	}
}
