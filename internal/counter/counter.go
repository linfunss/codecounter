// Package counter 负责读取文件、识别编程语言并聚合代码行数统计结果。
//
// 语言识别与计数规则来自 scc（https://github.com/boyter/scc），
// 本包在其之上完成目录遍历、并发调度与结果聚合。
package counter

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"sync"
	"time"

	"github.com/boyter/scc/v3/processor"

	"github.com/linfunss/codecounter/internal/ignore"
	"github.com/linfunss/codecounter/internal/scan"
)

// MaxFileSize 是默认的单文件大小上限（10 MiB）。
// 超过该大小的文件通常是数据文件或构建产物，跳过可以避免无谓的内存开销。
const MaxFileSize int64 = 10 * 1024 * 1024

// LanguageStat 表示单一语言的统计结果。
type LanguageStat struct {
	Language string `json:"language"`
	Files    int    `json:"files"`
	Code     int64  `json:"code"`
	Comment  int64  `json:"comment"`
	Blank    int64  `json:"blank"`
	Lines    int64  `json:"lines"`
	Bytes    int64  `json:"bytes"`
}

// Result 是一次统计的完整结果。
type Result struct {
	// Root 是统计根目录；当统计的是用户显式选择的文件时为空。
	Root string `json:"root"`
	// Duration 是本次统计的耗时。
	Duration time.Duration `json:"duration"`
	// Files 是成功计入统计的文件数。
	Files int `json:"files"`
	// Skipped 是被排除规则跳过的条目数。
	Skipped int `json:"skipped"`
	// Unsupported 是内容为二进制或语言无法识别、未计入统计的文件数。
	Unsupported int `json:"unsupported"`
	// Failed 是读取失败的文件数。
	Failed int `json:"failed"`
	// Bytes 是成功统计的文件总字节数。
	Bytes int64 `json:"bytes"`
	// Languages 是按代码行数降序排列的分语言统计。
	Languages []LanguageStat `json:"languages"`
	// Total 是所有语言的汇总。
	Total LanguageStat `json:"total"`
}

// Options 控制统计行为。
type Options struct {
	// Matcher 为 nil 时使用内置默认排除名单。
	Matcher *ignore.Matcher
	// MaxFileSize 为 0 时使用 MaxFileSize 常量。
	MaxFileSize int64
	// Workers 为 0 时使用 CPU 核心数。
	Workers int
}

// initOnce 保证 scc 的语言库只初始化一次。
var initOnce sync.Once

// initProcessor 加载 scc 的语言定义与注释规则。
// 该函数可以被并发调用，且只会真正执行一次。
func initProcessor() {
	initOnce.Do(processor.ProcessConstants)
}

// DetectLanguage 依据文件名与内容判断编程语言，无法判断时返回空字符串。
func DetectLanguage(name string, content []byte) string {
	initProcessor()

	possible, fallback := processor.DetectLanguage(name)
	if len(possible) == 0 {
		// 扩展名无法识别时退回 shebang 检测，覆盖无后缀的脚本文件
		if lang, err := processor.DetectSheBang(string(content)); err == nil {
			return lang
		}
		return ""
	}

	lang := processor.DetermineLanguage(name, fallback, possible, content)
	if lang == processor.SheBang {
		// 无后缀文件需要读取文件头才能确定具体语言
		cutoff := 200
		if len(content) < cutoff {
			cutoff = len(content)
		}
		detected, err := processor.DetectSheBang(string(content[:cutoff]))
		if err != nil {
			return ""
		}
		return detected
	}
	return lang
}

// CountDir 统计 root 目录（含子目录）中的代码行数。
func CountDir(ctx context.Context, root string, opts Options) (*Result, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("解析统计目录失败: %w", err)
	}

	info, err := os.Stat(absRoot)
	if err != nil {
		return nil, fmt.Errorf("访问统计目录失败: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("统计路径不是目录: %s", absRoot)
	}

	c, err := newCollector(ctx, opts)
	if err != nil {
		return nil, err
	}
	c.start()

	start := time.Now()
	stats, walkErr := scan.Walk(ctx, scan.Options{
		Root:        absRoot,
		Matcher:     c.matcher,
		MaxFileSize: c.maxFileSize,
	}, c.submit)
	c.stop()

	if walkErr != nil {
		return nil, fmt.Errorf("遍历统计目录失败: %w", walkErr)
	}

	res := c.result(absRoot, time.Since(start))
	res.Skipped += stats.Skipped
	return res, nil
}

// CountFiles 统计用户显式指定的文件列表。
//
// 用户手动选择的文件属于明确意图，不再套用排除规则。
func CountFiles(ctx context.Context, paths []string, opts Options) (*Result, error) {
	c, err := newCollector(ctx, opts)
	if err != nil {
		return nil, err
	}
	c.start()

	start := time.Now()
	entries, stats := scan.CollectFiles(paths, c.maxFileSize)
	for _, e := range entries {
		if submitErr := c.submit(e); submitErr != nil {
			c.stop()
			return nil, submitErr
		}
	}
	c.stop()

	res := c.result("", time.Since(start))
	res.Skipped += stats.Skipped
	return res, nil
}

// SortStats 按代码行数降序排列语言统计；相同时依次比较注释行数与语言名，
// 保证相同输入下的结果顺序稳定。
func SortStats(list []LanguageStat) {
	sort.SliceStable(list, func(i, j int) bool {
		if list[i].Code != list[j].Code {
			return list[i].Code > list[j].Code
		}
		if list[i].Comment != list[j].Comment {
			return list[i].Comment > list[j].Comment
		}
		return list[i].Language < list[j].Language
	})
}

// binarySampleSize 是判断文件是否为二进制时采样的字节数。
const binarySampleSize = 8000

// isBinary 通过探测 NUL 字节判断内容是否为二进制，
// 与版本控制系统常用的启发式一致。
func isBinary(content []byte) bool {
	sample := content
	if len(sample) > binarySampleSize {
		sample = sample[:binarySampleSize]
	}
	return bytes.IndexByte(sample, 0) >= 0
}

// collector 汇总并发统计过程中的中间结果。
type collector struct {
	ctx         context.Context
	matcher     *ignore.Matcher
	maxFileSize int64
	workers     int
	jobs        chan scan.Entry
	wg          sync.WaitGroup

	mu          sync.Mutex
	byLang      map[string]*LanguageStat
	files       int
	bytes       int64
	failed      int
	unsupported int
	binary      int
}

// newCollector 依据选项构建统计器，缺省值在此处补齐。
func newCollector(ctx context.Context, opts Options) (*collector, error) {
	matcher := opts.Matcher
	if matcher == nil {
		m, err := ignore.New(ignore.Options{UseDefaults: true})
		if err != nil {
			return nil, err
		}
		matcher = m
	}

	maxSize := opts.MaxFileSize
	if maxSize <= 0 {
		maxSize = MaxFileSize
	}

	workers := opts.Workers
	if workers <= 0 {
		workers = runtime.NumCPU()
	}

	return &collector{
		ctx:         ctx,
		matcher:     matcher,
		maxFileSize: maxSize,
		workers:     workers,
		jobs:        make(chan scan.Entry, 256),
		byLang:      make(map[string]*LanguageStat),
	}, nil
}

// start 启动工作协程。
func (c *collector) start() {
	for i := 0; i < c.workers; i++ {
		c.wg.Add(1)
		go func() {
			defer c.wg.Done()
			for e := range c.jobs {
				c.handle(e)
			}
		}()
	}
}

// submit 把文件投递给工作协程，上下文取消时立即返回。
func (c *collector) submit(e scan.Entry) error {
	select {
	case c.jobs <- e:
		return nil
	case <-c.ctx.Done():
		return c.ctx.Err()
	}
}

// stop 结束投递并等待所有工作协程退出。
func (c *collector) stop() {
	close(c.jobs)
	c.wg.Wait()
}

// handle 处理单个文件：读取内容、识别语言并累加统计结果。
func (c *collector) handle(e scan.Entry) {
	content, err := os.ReadFile(e.Path)
	if err != nil {
		c.markFailed()
		return
	}

	// 图片、可执行文件等二进制内容不属于代码，直接跳过而不计入「未识别」
	if isBinary(content) {
		c.markBinary()
		return
	}

	lang := DetectLanguage(filepath.Base(e.Path), content)
	if lang == "" {
		c.markUnsupported()
		return
	}

	job := &processor.FileJob{
		Filename:    filepath.Base(e.Path),
		Location:    e.Path,
		Symlocation: e.Path,
		Language:    lang,
		Content:     content,
		Bytes:       int64(len(content)),
	}
	processor.CountStats(job)

	// 二进制内容不计入统计
	if job.Binary {
		c.markUnsupported()
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	stat, ok := c.byLang[lang]
	if !ok {
		stat = &LanguageStat{Language: lang}
		c.byLang[lang] = stat
	}
	stat.Files++
	stat.Code += job.Code
	stat.Comment += job.Comment
	stat.Blank += job.Blank
	stat.Lines += job.Lines
	stat.Bytes += job.Bytes

	c.files++
	c.bytes += job.Bytes
}

// markBinary 记录一个被跳过的二进制文件。
func (c *collector) markBinary() {
	c.mu.Lock()
	c.binary++
	c.mu.Unlock()
}

// markUnsupported 记录一个语言无法识别的文本文件。
func (c *collector) markUnsupported() {
	c.mu.Lock()
	c.unsupported++
	c.mu.Unlock()
}

// markFailed 记录一个读取失败的文件。
func (c *collector) markFailed() {
	c.mu.Lock()
	c.failed++
	c.mu.Unlock()
}

// result 汇总最终结果，语言按代码行数降序排列。
func (c *collector) result(root string, elapsed time.Duration) *Result {
	c.mu.Lock()
	defer c.mu.Unlock()

	res := &Result{
		Root:        root,
		Duration:    elapsed,
		Files:       c.files,
		Skipped:     c.binary,
		Unsupported: c.unsupported,
		Failed:      c.failed,
		Bytes:       c.bytes,
		Languages:   make([]LanguageStat, 0, len(c.byLang)),
	}

	for _, stat := range c.byLang {
		res.Languages = append(res.Languages, *stat)
		res.Total.Files += stat.Files
		res.Total.Code += stat.Code
		res.Total.Comment += stat.Comment
		res.Total.Blank += stat.Blank
		res.Total.Lines += stat.Lines
		res.Total.Bytes += stat.Bytes
	}
	res.Total.Language = "合计"

	SortStats(res.Languages)
	return res
}
