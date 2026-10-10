// CodeCounter 是一个用于统计项目代码行数的小工具。
//
// 程序会扫描指定目录（缺省为当前工作目录），按语言聚合代码行数，
// 自动跳过 node_modules、.idea 之类的冗余目录，并以排行榜与条形图展示结果。
//
// 默认启动 Windows 原生图形界面；使用 --cli 或 --json 时改为命令行输出，
// 便于在脚本或 CI 中直接使用。
//go:generate go run github.com/akavel/rsrc@v0.10.2 -arch amd64 -manifest ../../winres/app.manifest -ico ../../winres/app.ico -o rsrc_windows_amd64.syso
//go:generate go run github.com/akavel/rsrc@v0.10.2 -arch arm64 -manifest ../../winres/app.manifest -ico ../../winres/app.ico -o rsrc_windows_arm64.syso

package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/linfunss/codecounter/internal/counter"
	"github.com/linfunss/codecounter/internal/format"
	"github.com/linfunss/codecounter/internal/ignore"
	"github.com/linfunss/codecounter/internal/ui"
)

// 以下变量在构建时通过 -ldflags 注入。
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

// barWidth 是命令行排行榜中条形图的最大宽度（以字符计）。
const barWidth = 40

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "错误: %v\n", err)
		os.Exit(1)
	}
}

// run 负责解析参数，并按模式执行统计或启动界面。
func run() error {
	var (
		pathFlag     = flag.String("path", "", "要统计的目录，缺省为当前工作目录")
		fileFlag     = flag.String("file", "", "只统计指定的文件，多个文件以分号分隔")
		cliMode      = flag.Bool("cli", false, "以命令行方式输出统计结果")
		jsonMode     = flag.Bool("json", false, "以 JSON 格式输出统计结果（隐含命令行模式）")
		noDefaults   = flag.Bool("no-default-ignore", false, "不启用内置的默认排除名单")
		extraRules   = flag.String("ignore", "", "附加排除规则，多条以分号分隔，支持 glob 与 ! 取反")
		useGitIgnore = flag.Bool("use-gitignore", true, "读取统计目录下的 .gitignore")
		maxSize      = flag.Int64("max-size", counter.MaxFileSize, "单个文件大小上限（字节），超过则跳过")
		workers      = flag.Int("workers", 0, "统计并发数，缺省为 CPU 核心数")
		topN         = flag.Int("top", 20, "命令行排行榜显示的语言数量，0 表示不限")
		showVersion  = flag.Bool("version", false, "显示版本信息后退出")
	)
	flag.Parse()

	if *showVersion {
		fmt.Printf("CodeCounter %s (commit %s, built %s)\n", version, commit, date)
		return nil
	}

	// 默认进入图形界面：排除规则由配置文件决定，--path 可覆盖初始统计目录
	if !*cliMode && !*jsonMode {
		if err := ui.Run(ui.Options{
			Version:    version,
			InitialDir: *pathFlag,
			Workers:    *workers,
		}); err != nil {
			ui.ShowError("CodeCounter 启动失败", err)
			return err
		}
		return nil
	}

	// 以下为命令行模式：目录缺省为当前工作目录，排除规则可由参数覆盖
	root := *pathFlag
	if root == "" {
		wd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("获取当前工作目录失败: %w", err)
		}
		root = wd
	}

	files := splitList(*fileFlag)
	matcher, err := buildMatcher(*noDefaults, *extraRules, *useGitIgnore, root, len(files) > 0)
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	opts := counter.Options{
		Matcher:     matcher,
		MaxFileSize: *maxSize,
		Workers:     *workers,
	}

	var res *counter.Result
	if len(files) > 0 {
		res, err = counter.CountFiles(ctx, files, opts)
	} else {
		res, err = counter.CountDir(ctx, root, opts)
	}
	if err != nil {
		return err
	}

	if *jsonMode {
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(res)
	}

	renderText(os.Stdout, res, *topN)
	return nil
}

// buildMatcher 依据参数构建排除规则匹配器。
func buildMatcher(noDefaults bool, extraRules string, useGitIgnore bool, root string, fileMode bool) (*ignore.Matcher, error) {
	opts := ignore.Options{
		UseDefaults: !noDefaults,
		Custom:      splitList(extraRules),
	}

	// 文件模式下用户已明确选择文件，不需要再套用 .gitignore
	if useGitIgnore && !fileMode && root != "" {
		gitignorePath := filepath.Join(root, ".gitignore")
		if info, err := os.Stat(gitignorePath); err == nil && !info.IsDir() {
			opts.GitIgnorePath = gitignorePath
		}
	}

	return ignore.New(opts)
}

// splitList 把分号分隔的参数拆分成列表，自动忽略空白项。
func splitList(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}

	parts := strings.Split(raw, ";")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if trimmed := strings.TrimSpace(p); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

// renderText 以文本排行榜的形式输出统计结果。
func renderText(w io.Writer, res *counter.Result, topN int) {
	root := res.Root
	if root == "" {
		root = "用户选择的文件"
	}

	fmt.Fprintf(w, "统计范围: %s\n", root)
	fmt.Fprintf(w, "已统计 %d 个文件 / %s，跳过 %d 项，未识别 %d 个，读取失败 %d 个，耗时 %s\n\n",
		res.Files, format.Bytes(res.Bytes), res.Skipped, res.Unsupported, res.Failed, res.Duration.Round(1e6))

	stats := res.Languages
	truncated := false
	if topN > 0 && len(stats) > topN {
		stats = stats[:topN]
		truncated = true
	}

	fmt.Fprintln(w, "排行榜")
	fmt.Fprintf(w, "%-4s %-20s %8s %12s %12s %10s\n", "排名", "语言", "文件数", "代码行", "注释行", "空行")
	fmt.Fprintln(w, strings.Repeat("-", 72))
	for i, stat := range stats {
		fmt.Fprintf(w, "%-4d %-20s %8d %12s %12s %10s\n",
			i+1, stat.Language, stat.Files,
			format.Int(stat.Code), format.Int(stat.Comment), format.Int(stat.Blank))
	}
	if len(stats) == 0 {
		fmt.Fprintln(w, "（没有可统计的代码文件）")
	}
	if truncated {
		fmt.Fprintf(w, "…… 其余 %d 种语言未显示，可使用 -top 0 查看全部\n", len(res.Languages)-len(stats))
	}

	fmt.Fprintf(w, "\n%-4s %-20s %8d %12s %12s %10s\n", "", "合计", res.Total.Files,
		format.Int(res.Total.Code), format.Int(res.Total.Comment), format.Int(res.Total.Blank))

	if len(stats) > 0 {
		fmt.Fprintln(w, "\n代码行数分布")
		renderBars(w, stats)
	}
}

// renderBars 绘制横向条形图，长度按代码行数相对最大值等比缩放。
func renderBars(w io.Writer, stats []counter.LanguageStat) {
	var maxCode int64
	for _, stat := range stats {
		if stat.Code > maxCode {
			maxCode = stat.Code
		}
	}
	if maxCode <= 0 {
		return
	}

	// 语言名对齐宽度，避免条形图起始位置参差不齐
	nameWidth := 0
	for _, stat := range stats {
		if n := len([]rune(stat.Language)); n > nameWidth {
			nameWidth = n
		}
	}
	if nameWidth > 24 {
		nameWidth = 24
	}

	for _, stat := range stats {
		filled := int(stat.Code * int64(barWidth) / maxCode)
		if stat.Code > 0 && filled == 0 {
			filled = 1
		}
		bar := strings.Repeat("█", filled) + strings.Repeat(" ", barWidth-filled)
		fmt.Fprintf(w, "%-*s %s %s\n", nameWidth, truncate(stat.Language, nameWidth), bar, format.Int(stat.Code))
	}
}

// truncate 按显示宽度截断字符串，超出部分以省略号结尾。
func truncate(s string, width int) string {
	runes := []rune(s)
	if len(runes) <= width {
		return s
	}
	if width <= 1 {
		return string(runes[:width])
	}
	return string(runes[:width-1]) + "…"
}
