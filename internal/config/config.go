// Package config 负责 CodeCounter 用户配置的读写。
//
// 配置以 JSON 形式保存在 %APPDATA%\CodeCounter\config.json。
// 文件缺失时使用内置默认值；文件损坏时同样回退到默认值，并把错误
// 交给调用方提示用户，以保证程序仍可正常启动。
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/linfunss/codecounter/internal/ignore"
)

// DirMode 表示启动时默认统计哪个目录。
type DirMode string

const (
	// DirModeWorkingDir 统计启动程序时的工作目录。
	DirModeWorkingDir DirMode = "cwd"
	// DirModeLast 统计上一次使用过的目录。
	DirModeLast DirMode = "last"
	// DirModeFixed 始终统计配置中指定的固定目录。
	DirModeFixed DirMode = "fixed"
)

// Version 是当前配置结构版本，便于后续做兼容处理。
const Version = 1

// 界面缩放的可用区间。
const (
	// MinScale 是最小界面缩放系数。
	MinScale = 0.8
	// MaxScale 是最大界面缩放系数。
	MaxScale = 1.6
)

// appDirName 是配置在用户配置目录下的子目录名。
const appDirName = "CodeCounter"

// EnvConfigDir 是覆盖配置目录的环境变量名。
//
// 设置后配置会存放在该目录，便于便携部署，也便于测试时隔离配置。
const EnvConfigDir = "CODECOUNTER_CONFIG_DIR"

// Config 是持久化的用户配置。
type Config struct {
	Version int `json:"version"`

	// DefaultDirMode 决定启动时统计哪个目录。
	DefaultDirMode DirMode `json:"defaultDirMode"`
	// FixedDir 在 DefaultDirMode 为 fixed 时生效。
	FixedDir string `json:"fixedDir"`
	// LastDir 记录上一次统计的目录，供 last 模式使用。
	LastDir string `json:"lastDir"`

	// UseDefaultExcludes 控制是否启用内置排除名单。
	UseDefaultExcludes bool `json:"useDefaultExcludes"`
	// CustomExcludes 是用户自定义排除规则，一行一条。
	CustomExcludes []string `json:"customExcludes"`
	// UseGitIgnore 控制是否读取统计目录下的 .gitignore。
	UseGitIgnore bool `json:"useGitIgnore"`

	// ShowLanguageIcons 控制条形图是否绘制语言图标。
	ShowLanguageIcons bool `json:"showLanguageIcons"`
	// MaxBars 是条形图最多显示的语言数量，0 表示不限。
	MaxBars int `json:"maxBars"`
	// ShowPercent 控制条形图是否显示代码行数占比。
	ShowPercent bool `json:"showPercent"`
	// Scale 是界面缩放系数，1.0 为标准大小。
	Scale float64 `json:"scale"`
}

// Defaults 返回内置默认配置。
func Defaults() Config {
	return Config{
		Version:            Version,
		DefaultDirMode:     DirModeWorkingDir,
		UseDefaultExcludes: true,
		UseGitIgnore:       true,
		ShowLanguageIcons:  true,
		MaxBars:            20,
		ShowPercent:        true,
		Scale:              1.0,
	}
}

// Dir 返回配置文件所在目录。
//
// 默认使用 %APPDATA%\CodeCounter；若设置了 CODECOUNTER_CONFIG_DIR，
// 则改用该目录。
func Dir() (string, error) {
	if override := strings.TrimSpace(os.Getenv(EnvConfigDir)); override != "" {
		return override, nil
	}

	base, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("获取用户配置目录失败: %w", err)
	}
	return filepath.Join(base, appDirName), nil
}

// Path 返回配置文件的完整路径。
func Path() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.json"), nil
}

// Load 读取配置。
//
// 返回的 error 表示读取或解析过程中出现的问题；此时仍会返回默认配置，
// 调用方可以据此提示用户，但不必中断程序。
func Load() (Config, error) {
	path, err := Path()
	if err != nil {
		return Defaults(), err
	}

	data, readErr := os.ReadFile(path)
	if readErr != nil {
		if errors.Is(readErr, fs.ErrNotExist) {
			return Defaults(), nil
		}
		return Defaults(), fmt.Errorf("读取配置文件失败: %w", readErr)
	}

	cfg := Defaults()
	if unmarshalErr := json.Unmarshal(data, &cfg); unmarshalErr != nil {
		return Defaults(), fmt.Errorf("解析配置文件失败: %w", unmarshalErr)
	}
	cfg.Normalize()
	return cfg, nil
}

// Save 把配置写入磁盘，目录不存在时自动创建。
func (c Config) Save() error {
	path, err := Path()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("创建配置目录失败: %w", err)
	}

	c.Version = Version
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化配置失败: %w", err)
	}
	data = append(data, '\n')

	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("写入配置文件失败: %w", err)
	}
	return nil
}

// NewMatcher 依据配置为指定统计目录构建排除规则匹配器。
func (c Config) NewMatcher(root string) (*ignore.Matcher, error) {
	opts := ignore.Options{
		UseDefaults: c.UseDefaultExcludes,
		Custom:      c.CustomExcludes,
	}

	if c.UseGitIgnore && root != "" {
		path := filepath.Join(root, ".gitignore")
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			opts.GitIgnorePath = path
		}
	}

	return ignore.New(opts)
}

// InitialDir 返回启动时应当统计的目录。
//
// fixed 与 last 模式指向的目录不存在时，回退到当前工作目录。
func (c Config) InitialDir() string {
	switch c.DefaultDirMode {
	case DirModeLast:
		if isDir(c.LastDir) {
			return c.LastDir
		}
	case DirModeFixed:
		if isDir(c.FixedDir) {
			return c.FixedDir
		}
	}

	if wd, err := os.Getwd(); err == nil {
		return wd
	}
	return ""
}

// Normalize 修正非法或缺失的字段，保证配置始终可用。
func (c *Config) Normalize() {
	switch c.DefaultDirMode {
	case DirModeWorkingDir, DirModeLast, DirModeFixed:
	default:
		c.DefaultDirMode = DirModeWorkingDir
	}

	if c.MaxBars < 0 {
		c.MaxBars = Defaults().MaxBars
	}

	// 缩放过小会看不清、过大则会挤掉内容，这里限制在可用区间内
	if c.Scale < MinScale || c.Scale > MaxScale {
		c.Scale = Defaults().Scale
	}

	rules := make([]string, 0, len(c.CustomExcludes))
	for _, rule := range c.CustomExcludes {
		if trimmed := strings.TrimSpace(rule); trimmed != "" {
			rules = append(rules, trimmed)
		}
	}
	c.CustomExcludes = rules
}

// isDir 判断路径是否为存在的目录。
func isDir(path string) bool {
	if path == "" {
		return false
	}
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
