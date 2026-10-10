// Package ignore 负责判断某个路径是否应当从代码行数统计中跳过。
//
// 规则来源有三部分：
//  1. 内置默认名单（依赖目录、缓存、构建产物、编辑器数据等）
//  2. 用户自定义规则，支持 glob 通配与 ! 取反
//  3. 可选的 .gitignore 文件
//
// 匹配以"相对统计根目录的路径"为单位。目录一旦命中即整棵子树跳过，
// 因此 ! 取反无法重新纳入已被跳过目录中的文件，这一点与主流统计工具一致。
package ignore

import (
	"fmt"
	"path"
	"strings"

	gitignore "github.com/sabhiram/go-gitignore"
)

// defaultRules 是内置排除名单。
//
// 不含 / 的条目按"任意层级的同名文件或目录"匹配，例如 node_modules 会命中
// 任意深度的 node_modules 目录；含 / 的条目按相对路径匹配，例如 src/generated。
var defaultRules = []string{
	// 版本控制
	".git", ".svn", ".hg", ".bzr",

	// 依赖与包管理
	"node_modules", "bower_components", "jspm_packages", "vendor", "Pods",
	"Carthage", "packages", ".bundle", ".pub-cache", ".cargo", ".gradle",
	"site-packages",

	// 缓存与虚拟环境
	"__pycache__", ".pytest_cache", ".mypy_cache", ".ruff_cache", ".tox",
	".venv", "venv", ".cache", ".parcel-cache", ".turbo", ".sass-cache",
	".terraform", ".next", ".nuxt", ".output", ".angular", ".ipynb_checkpoints",

	// 构建产物
	"dist", "build", "out", "output", "target", "bin", "obj",
	"Debug", "Release", "cmake-build-debug", "cmake-build-release",
	".dart_tool", "DerivedData",

	// 编辑器与 IDE
	".idea", ".vscode", ".vs", ".fleet", ".history", ".settings",

	// 覆盖率与临时文件
	"coverage", ".nyc_output", "tmp", "temp", ".tmp", ".Trash",

	// 常见工程配置文件，属于配置而非源码
	".gitignore", ".gitattributes", ".gitmodules", ".editorconfig",
	".dockerignore", ".npmignore", ".eslintignore", ".prettierignore",
	".mailmap", ".gitkeep",

	// 自动生成的依赖锁定文件，体量大且非手写代码
	"go.sum", "package-lock.json", "yarn.lock", "pnpm-lock.yaml",
	"Cargo.lock", "poetry.lock", "composer.lock", "Gemfile.lock",
	"packages.lock.json", "Podfile.lock",
}

// DefaultRules 返回内置默认排除规则的副本，调用方可以安全修改。
func DefaultRules() []string {
	out := make([]string, len(defaultRules))
	copy(out, defaultRules)
	return out
}

// rule 表示一条已解析的排除规则。
type rule struct {
	pattern string // 去掉 ! 前缀与结尾 / 之后的模式
	negate  bool   // 形如 !pattern，命中后重新纳入统计
	dirOnly bool   // 形如 pattern/，只匹配目录
}

// Options 控制 Matcher 的构建。
type Options struct {
	// UseDefaults 为 true 时启用内置默认排除名单。
	UseDefaults bool
	// Custom 是用户自定义规则，一行一条，支持 glob 通配与 ! 取反。
	Custom []string
	// GitIgnorePath 非空时启用该文件中的 .gitignore 规则。
	GitIgnorePath string
}

// Matcher 按规则判断路径是否应被跳过。
// 构建完成后 Matcher 可以安全地被多个 goroutine 并发使用。
type Matcher struct {
	rules []rule
	git   *gitignore.GitIgnore
}

// New 根据给定选项构建一个 Matcher。
func New(opts Options) (*Matcher, error) {
	m := &Matcher{}

	if opts.UseDefaults {
		for _, p := range defaultRules {
			m.rules = append(m.rules, parseRule(p))
		}
	}

	for _, raw := range opts.Custom {
		r, ok := parseLine(raw)
		if !ok {
			continue
		}
		m.rules = append(m.rules, r)
	}

	if opts.GitIgnorePath != "" {
		gi, err := gitignore.CompileIgnoreFile(opts.GitIgnorePath)
		if err != nil {
			return nil, fmt.Errorf("读取 .gitignore 失败: %w", err)
		}
		m.git = gi
	}

	return m, nil
}

// RuleCount 返回生效的规则数量，便于界面展示。
func (m *Matcher) RuleCount() int {
	if m == nil {
		return 0
	}
	return len(m.rules)
}

// Skip 判断相对路径 rel 是否应被跳过。
// rel 使用 / 作为分隔符，isDir 表示该路径是否为目录。
func (m *Matcher) Skip(rel string, isDir bool) bool {
	if m == nil {
		return false
	}

	rel = normalizeRel(rel)
	if rel == "" {
		return false
	}

	base := path.Base(rel)

	if m.git != nil {
		// .gitignore 中的 "dir/" 写法需要带结尾斜杠才能命中目录本身，
		// 因此目录要额外用 "dir/" 形式再匹配一次
		if m.git.MatchesPath(rel) {
			return true
		}
		if isDir && m.git.MatchesPath(rel+"/") {
			return true
		}
	}

	skip := false
	for _, r := range m.rules {
		if r.dirOnly && !isDir {
			continue
		}
		if r.match(rel, base) {
			skip = !r.negate
		}
	}
	return skip
}

// match 判断单条规则是否命中给定路径。
func (r rule) match(rel, base string) bool {
	// 不含 / 的模式只匹配任意层级的名字
	if !strings.Contains(r.pattern, "/") {
		if r.pattern == base {
			return true
		}
		ok, err := path.Match(r.pattern, base)
		return err == nil && ok
	}

	// 含 / 的模式按相对路径匹配，同时兼容 **/ 前缀
	pattern := r.pattern
	if strings.HasPrefix(pattern, "**/") {
		trimmed := strings.TrimPrefix(pattern, "**/")
		if ok, err := path.Match(trimmed, rel); err == nil && ok {
			return true
		}
		pattern = trimmed
	}

	if ok, err := path.Match(pattern, rel); err == nil && ok {
		return true
	}
	// 作为目录前缀匹配，例如 src/generated 命中 src/generated/a.go
	return strings.HasPrefix(rel, pattern+"/")
}

// parseRule 解析一条来自内置名单的规则。
func parseRule(pattern string) rule {
	r := rule{pattern: pattern}
	if strings.HasSuffix(r.pattern, "/") {
		r.dirOnly = true
		r.pattern = strings.TrimSuffix(r.pattern, "/")
	}
	return r
}

// parseLine 解析一行用户规则，空行与 # 注释会被忽略。
func parseLine(raw string) (rule, bool) {
	line := strings.TrimSpace(raw)
	if line == "" || strings.HasPrefix(line, "#") {
		return rule{}, false
	}

	line = strings.ReplaceAll(line, "\\", "/")
	line = strings.TrimPrefix(line, "./")

	r := rule{}
	if strings.HasPrefix(line, "!") {
		r.negate = true
		line = strings.TrimPrefix(line, "!")
	}
	if strings.HasSuffix(line, "/") {
		r.dirOnly = true
		line = strings.TrimSuffix(line, "/")
	}
	if line == "" {
		return rule{}, false
	}

	r.pattern = line
	return r, true
}

// normalizeRel 统一相对路径的书写形式，便于匹配。
func normalizeRel(rel string) string {
	rel = strings.ReplaceAll(rel, "\\", "/")
	rel = strings.TrimPrefix(rel, "./")
	rel = strings.Trim(rel, "/")
	if rel == "" {
		return ""
	}
	rel = path.Clean(rel)
	if rel == "." {
		return ""
	}
	return rel
}
