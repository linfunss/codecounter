// Package scan 负责遍历目录，并按排除规则筛选出待统计的文件。
package scan

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/linfunss/codecounter/internal/ignore"
)

// Entry 表示一个待统计的文件。
type Entry struct {
	// Path 是文件在磁盘上的绝对路径。
	Path string
	// RelPath 是相对统计根目录的路径，统一使用 / 作为分隔符。
	RelPath string
	// Size 是文件字节数。
	Size int64
}

// Options 控制目录遍历行为。
type Options struct {
	// Root 是统计根目录。
	Root string
	// Matcher 为 nil 时不做任何排除。
	Matcher *ignore.Matcher
	// MaxFileSize 大于 0 时，超过该字节数的文件会被跳过。
	MaxFileSize int64
}

// Stats 记录一次遍历的统计信息。
type Stats struct {
	// Files 是进入统计队列的文件数。
	Files int
	// Dirs 是访问过的目录数。
	Dirs int
	// Skipped 是被排除规则或读取失败跳过的条目数。
	Skipped int
	// Bytes 是进入统计队列的文件总字节数。
	Bytes int64
}

// Walk 遍历 opts.Root，把每个未被排除的常规文件交给 visit 处理。
//
// 单个路径读取失败不会中断整体遍历，只会累加 Skipped 计数；
// 但如果 visit 返回错误或上下文被取消，遍历会立即结束。
func Walk(ctx context.Context, opts Options, visit func(Entry) error) (Stats, error) {
	var stats Stats

	root := opts.Root
	if root == "" {
		root = "."
	}

	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}

		// 无法访问的条目直接跳过，避免因个别权限问题中断整次统计
		if err != nil {
			stats.Skipped++
			if d != nil && d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}

		rel, relErr := filepath.Rel(root, p)
		if relErr != nil {
			stats.Skipped++
			return nil
		}
		if rel == "." {
			// 根目录本身不参与规则匹配
			stats.Dirs++
			return nil
		}
		rel = filepath.ToSlash(rel)

		if d.IsDir() {
			if opts.Matcher != nil && opts.Matcher.Skip(rel, true) {
				stats.Skipped++
				return fs.SkipDir
			}
			stats.Dirs++
			return nil
		}

		if !d.Type().IsRegular() {
			stats.Skipped++
			return nil
		}

		if opts.Matcher != nil && opts.Matcher.Skip(rel, false) {
			stats.Skipped++
			return nil
		}

		info, infoErr := d.Info()
		if infoErr != nil {
			stats.Skipped++
			return nil
		}
		if opts.MaxFileSize > 0 && info.Size() > opts.MaxFileSize {
			stats.Skipped++
			return nil
		}

		stats.Files++
		stats.Bytes += info.Size()

		if visit == nil {
			return nil
		}
		return visit(Entry{Path: p, RelPath: rel, Size: info.Size()})
	})

	return stats, err
}

// CollectFiles 把用户显式指定的文件整理成待统计条目。
//
// 用户手动选中的文件属于明确意图，因此不再套用排除规则，
// 但仍会跳过目录项、不存在的路径以及超过大小上限的文件。
func CollectFiles(paths []string, maxFileSize int64) ([]Entry, Stats) {
	entries := make([]Entry, 0, len(paths))
	var stats Stats

	for _, p := range paths {
		info, err := os.Stat(p)
		if err != nil || info.IsDir() {
			stats.Skipped++
			continue
		}
		if maxFileSize > 0 && info.Size() > maxFileSize {
			stats.Skipped++
			continue
		}

		abs, err := filepath.Abs(p)
		if err != nil {
			stats.Skipped++
			continue
		}

		stats.Files++
		stats.Bytes += info.Size()
		entries = append(entries, Entry{
			Path:    abs,
			RelPath: filepath.ToSlash(filepath.Base(abs)),
			Size:    info.Size(),
		})
	}

	return entries, stats
}
