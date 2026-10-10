// Package ui 提供 CodeCounter 的 Windows 原生图形界面。
//
// 界面使用 lxn/walk，基于系统原生 Win32 控件，因此外观与系统主题一致，
// 不依赖 WebView 或任何运行时。
package ui

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/lxn/walk"
	decl "github.com/lxn/walk/declarative"

	"github.com/linfunss/codecounter/internal/config"
	"github.com/linfunss/codecounter/internal/counter"
	"github.com/linfunss/codecounter/internal/format"
)

// Options 控制主窗口的启动行为。
type Options struct {
	// Version 显示在窗口标题上。
	Version string
	// InitialDir 非空时覆盖配置里的默认统计目录。
	InitialDir string
	// Workers 为 0 时使用 CPU 核心数。
	Workers int
}

// App 是 CodeCounter 的主窗口。
type App struct {
	opts  Options
	model *leaderboardModel
	chart *barChart

	window   *walk.MainWindow
	pathEdit *walk.LineEdit
	table    *walk.TableView
	status   *walk.StatusBarItem

	ctx    context.Context
	cancel context.CancelFunc

	mu       sync.Mutex
	cfg      config.Config
	root     string
	files    []string
	scanning bool
	last     *counter.Result
}

// Run 创建主窗口并进入消息循环，直到用户关闭窗口。
func Run(opts Options) error {
	cfg, loadErr := config.Load()

	app := &App{
		opts:  opts,
		cfg:   cfg,
		model: &leaderboardModel{},
		chart: newBarChart(),
	}
	app.chart.SetOptions(cfg.ShowLanguageIcons, cfg.MaxBars)
	app.ctx, app.cancel = context.WithCancel(context.Background())

	if err := app.create(); err != nil {
		return err
	}

	app.window.Starting().Attach(func() {
		if loadErr != nil {
			walk.MsgBox(app.window, "配置读取失败",
				loadErr.Error()+"\n\n已改用默认设置继续运行。", walk.MsgBoxIconWarning)
		}
		// 窗口显示后立即统计，避免界面因等待遍历而卡住

		go app.scanDir(app.initialDir())
	})
	app.window.Closing().Attach(func(canceled *bool, reason walk.CloseReason) {
		app.cancel()
	})

	app.window.Run()
	return nil
}

// create 构建主窗口及其控件。
func (a *App) create() error {
	err := decl.MainWindow{
		AssignTo: &a.window,
		Title:    fmt.Sprintf("CodeCounter %s", a.opts.Version),
		MinSize:  decl.Size{Width: 880, Height: 560},
		Size:     decl.Size{Width: 1040, Height: 720},
		Layout:   decl.VBox{},
		ToolBar: decl.ToolBar{
			// 默认样式为仅显示图标，这里改用文字按钮
			ButtonStyle: decl.ToolBarButtonTextOnly,
			Items: []decl.MenuItem{
				decl.Action{Text: "选择目录", OnTriggered: func() { a.chooseDir() }},
				decl.Action{Text: "选择文件", OnTriggered: func() { a.chooseFiles() }},
				decl.Action{Text: "重新统计", OnTriggered: func() { a.rescan() }},
				decl.Action{Text: "导出 JSON", OnTriggered: func() { a.exportJSON() }},
				decl.Action{Text: "设置", OnTriggered: func() { a.openPrefs() }},
			},
		},
		Children: []decl.Widget{
			decl.Composite{
				Layout: decl.HBox{},
				Children: []decl.Widget{
					decl.Label{Text: "统计范围:"},
					decl.LineEdit{AssignTo: &a.pathEdit, ReadOnly: true, StretchFactor: 1},
					decl.PushButton{Text: "浏览…", OnClicked: func() { a.chooseDir() }},
				},
			},
			decl.GroupBox{
				Title:         "排行榜",
				Layout:        decl.VBox{},
				MinSize:       decl.Size{Height: 180},
				StretchFactor: 3,
				Children: []decl.Widget{
					decl.TableView{
						AssignTo:         &a.table,
						Model:            a.model,
						AlternatingRowBG: true,
						ColumnsOrderable: false,
						StretchFactor:    1,
						Columns: []decl.TableViewColumn{
							{Title: "排名", Width: 56, Alignment: decl.AlignFar},
							{Title: "语言", Width: 170},
							{Title: "文件数", Width: 84, Alignment: decl.AlignFar},
							{Title: "代码行", Width: 100, Alignment: decl.AlignFar},
							{Title: "注释行", Width: 100, Alignment: decl.AlignFar},
							{Title: "空行", Width: 92, Alignment: decl.AlignFar},
							{Title: "总行数", Width: 100, Alignment: decl.AlignFar},
						},
					},
				},
			},
			decl.GroupBox{
				Title:   "代码行数分布",
				Layout:  decl.VBox{},
				MinSize: decl.Size{Height: 330},
				MaxSize: decl.Size{Height: 330},
				Children: []decl.Widget{
					decl.CustomWidget{
						AssignTo:            &a.chart.widget,
						StretchFactor:       1,
						PaintMode:           decl.PaintBuffered,
						InvalidatesOnResize: true,
						Paint:               a.chart.paint,
					},
				},
			},
		},
		StatusBarItems: []decl.StatusBarItem{
			{AssignTo: &a.status, Text: "准备就绪"},
		},
	}.Create()
	if err != nil {
		return err
	}

	if icon := appIcon(); icon != nil {
		_ = a.window.SetIcon(icon)
	}
	return nil
}

// initialDir 返回启动时应统计的目录。
func (a *App) initialDir() string {
	if a.opts.InitialDir != "" {
		return a.opts.InitialDir
	}
	return a.currentConfig().InitialDir()
}

// scanDir 在后台统计目录，并在完成后更新界面。
func (a *App) scanDir(root string) {
	if root == "" {
		root = a.currentConfig().InitialDir()
	}
	if root == "" {
		a.reportError("无法确定统计目录", fmt.Errorf("当前工作目录不可用"))
		return
	}

	if !a.beginScan(root, nil) {
		return
	}

	opts, err := a.counterOptions(root)
	if err != nil {
		a.finishScan(nil, err)
		return
	}

	res, err := counter.CountDir(a.ctx, root, opts)
	if err == nil {
		a.rememberDir(root)
	}
	a.finishScan(res, err)
}

// scanFiles 在后台统计用户显式选择的文件。
func (a *App) scanFiles(files []string) {
	if !a.beginScan("", files) {
		return
	}

	opts, err := a.counterOptions("")
	if err != nil {
		a.finishScan(nil, err)
		return
	}

	res, err := counter.CountFiles(a.ctx, files, opts)
	a.finishScan(res, err)
}

// beginScan 记录本次统计范围，返回 false 表示已有统计在执行。
func (a *App) beginScan(root string, files []string) bool {
	a.mu.Lock()
	if a.scanning {
		a.mu.Unlock()
		a.runOnUI(func() { a.setStatus("正在统计，请稍候…") })
		return false
	}
	a.scanning = true
	a.root = root
	a.files = files
	a.mu.Unlock()

	label := root
	if len(files) > 0 {
		label = fmt.Sprintf("已选择 %d 个文件", len(files))
	}

	a.runOnUI(func() {
		_ = a.pathEdit.SetText(label)
		a.setStatus("正在统计…")
	})
	return true
}

// finishScan 把统计结果写回界面。
func (a *App) finishScan(res *counter.Result, err error) {
	a.mu.Lock()
	a.scanning = false
	if err == nil {
		a.last = res
	}
	a.mu.Unlock()

	if err != nil {
		a.reportError("统计失败", err)
		return
	}

	a.runOnUI(func() {
		a.model.SetRows(res.Languages)
		a.chart.SetRows(res.Languages)

		summary := fmt.Sprintf("已统计 %d 个文件 · 代码 %s 行 · 注释 %s 行 · 跳过 %d 项 · 耗时 %s",
			res.Files, format.Int(res.Total.Code), format.Int(res.Total.Comment),
			res.Skipped, res.Duration.Round(time.Millisecond))
		a.setStatus(summary)
	})
}

// chooseDir 让用户选择要统计的目录。
func (a *App) chooseDir() {
	dlg := walk.FileDialog{
		Title:          "选择要统计的目录",
		InitialDirPath: a.currentRoot(),
	}

	accepted, err := dlg.ShowBrowseFolder(a.window)
	if err != nil {
		a.reportError("选择目录失败", err)
		return
	}
	if !accepted || dlg.FilePath == "" {
		return
	}

	go a.scanDir(dlg.FilePath)
}

// chooseFiles 让用户选择要统计的文件。
func (a *App) chooseFiles() {
	dlg := walk.FileDialog{
		Title:          "选择要统计的文件",
		InitialDirPath: a.currentRoot(),
		Filter:         "所有文件 (*.*)|*.*",
	}

	accepted, err := dlg.ShowOpenMultiple(a.window)
	if err != nil {
		a.reportError("选择文件失败", err)
		return
	}
	if !accepted || len(dlg.FilePaths) == 0 {
		return
	}

	go a.scanFiles(dlg.FilePaths)
}

// rescan 按当前统计范围重新统计一次。
func (a *App) rescan() {
	a.mu.Lock()
	root := a.root
	files := append([]string(nil), a.files...)
	a.mu.Unlock()

	if len(files) > 0 {
		go a.scanFiles(files)
		return
	}
	go a.scanDir(root)
}

// openPrefs 打开设置对话框并把结果应用到当前会话。
func (a *App) openPrefs() {
	updated, accepted, err := showPrefs(a.window, a.opts.Version, a.currentConfig())
	if err != nil {
		a.reportError("打开设置失败", err)
		return
	}
	if !accepted {
		return
	}

	if err := updated.Save(); err != nil {
		a.reportError("保存设置失败", err)
		return
	}

	a.mu.Lock()
	a.cfg = updated
	a.mu.Unlock()

	a.chart.SetOptions(updated.ShowLanguageIcons, updated.MaxBars)
	a.rescan()
}

// exportJSON 把最近一次统计结果导出为 JSON 文件。
func (a *App) exportJSON() {
	a.mu.Lock()
	res := a.last
	a.mu.Unlock()

	if res == nil {
		walk.MsgBox(a.window, "导出 JSON", "还没有统计数据，请先执行一次统计。", walk.MsgBoxIconInformation)
		return
	}

	dlg := walk.FileDialog{
		Title:    "导出统计结果",
		Filter:   "JSON 文件 (*.json)|*.json|所有文件 (*.*)|*.*",
		FilePath: "codecounter-result.json",
	}

	accepted, err := dlg.ShowSave(a.window)
	if err != nil {
		a.reportError("导出失败", err)
		return
	}
	if !accepted || dlg.FilePath == "" {
		return
	}

	data, err := json.MarshalIndent(res, "", "  ")
	if err != nil {
		a.reportError("导出失败", err)
		return
	}
	if err := os.WriteFile(dlg.FilePath, data, 0o644); err != nil {
		a.reportError("写入文件失败", err)
		return
	}

	walk.MsgBox(a.window, "导出完成", "统计结果已导出到：\n"+dlg.FilePath, walk.MsgBoxIconInformation)
}

// counterOptions 依据当前配置构建统计参数。
func (a *App) counterOptions(root string) (counter.Options, error) {
	matcher, err := a.currentConfig().NewMatcher(root)
	if err != nil {
		return counter.Options{}, err
	}
	return counter.Options{Matcher: matcher, Workers: a.opts.Workers}, nil
}

// rememberDir 记录最近统计的目录，供「上一次使用的目录」策略使用。
func (a *App) rememberDir(root string) {
	a.mu.Lock()
	if a.cfg.LastDir == root {
		a.mu.Unlock()
		return
	}
	a.cfg.LastDir = root
	snapshot := a.cfg
	a.mu.Unlock()

	if err := snapshot.Save(); err != nil {
		a.runOnUI(func() { a.setStatus("配置保存失败: " + err.Error()) })
	}
}

// currentConfig 返回当前配置的副本。
func (a *App) currentConfig() config.Config {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.cfg
}

// currentRoot 返回当前统计目录，用于文件对话框的初始位置。
func (a *App) currentRoot() string {
	a.mu.Lock()
	root := a.root
	a.mu.Unlock()

	if root != "" {
		return root
	}
	if wd, err := os.Getwd(); err == nil {
		return wd
	}
	return ""
}

// runOnUI 把函数投递到界面线程执行。
func (a *App) runOnUI(f func()) {
	if a.window == nil {
		return
	}
	a.window.Synchronize(f)
}

// setStatus 更新状态栏文本。
func (a *App) setStatus(text string) {
	if a.status == nil {
		return
	}
	// 状态栏更新失败没有补救手段，忽略返回值
	_ = a.status.SetText(text)
}

// reportError 在主线程弹出错误提示。
func (a *App) reportError(title string, err error) {
	a.runOnUI(func() {
		a.setStatus(title)
		walk.MsgBox(a.window, title, err.Error(), walk.MsgBoxIconError)
	})
}
