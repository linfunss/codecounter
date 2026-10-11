package ui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/lxn/walk"
	decl "github.com/lxn/walk/declarative"

	"github.com/linfunss/codecounter/internal/config"
	"github.com/linfunss/codecounter/internal/ignore"
)

// dirModeLabels 是默认统计目录策略在下拉框中的显示顺序。
var dirModeLabels = []string{
	"启动程序时的工作目录",
	"上一次使用的目录",
	"固定目录",
}

// dirModeValues 与 dirModeLabels 一一对应。
var dirModeValues = []config.DirMode{
	config.DirModeWorkingDir,
	config.DirModeLast,
	config.DirModeFixed,
}

// maxBarsLabels 是条形图显示数量上限的候选项。
var maxBarsLabels = []string{"10", "20", "30", "50", "不限"}

// maxBarsValues 与 maxBarsLabels 一一对应，0 表示不限。
var maxBarsValues = []int{10, 20, 30, 50, 0}

// scaleLabels 是界面缩放系数的候选项。
var scaleLabels = []string{"80%", "90%", "100%", "110%", "125%", "150%"}

// scaleValues 与 scaleLabels 一一对应。
var scaleValues = []float64{0.8, 0.9, 1.0, 1.1, 1.25, 1.5}

// prefsDialog 是设置对话框，负责展示配置项并在用户确认时回传新配置。
type prefsDialog struct {
	owner   walk.Form
	version string
	current config.Config
	// root 是当前统计目录，用于把用户选择的路径转换为相对规则。
	root string
	// apply 由调用方提供，负责保存并立即生效。
	apply func(config.Config) error

	dlg *walk.Dialog

	dirModeCombo *walk.ComboBox
	fixedDirEdit *walk.LineEdit

	useDefaultsCheck  *walk.CheckBox
	useGitIgnoreCheck *walk.CheckBox
	rulesEdit         *walk.TextEdit
	staleLabel        *walk.Label

	showIconsCheck   *walk.CheckBox
	showPercentCheck *walk.CheckBox
	scaleCombo       *walk.ComboBox

	applyButton  *walk.PushButton
	saved        config.Config
	okButton     *walk.PushButton
	cancelButton *walk.PushButton
}

// showPrefs 打开设置对话框。
//
// 用户在对话框中点击「应用」或「确定」时，apply 会立即被调用，
// 因此不需要退出对话框即可让设置生效。apply 返回错误时对话框保持打开。
func showPrefs(owner walk.Form, version string, current config.Config, root string, apply func(config.Config) error) error {
	p := &prefsDialog{
		owner:   owner,
		version: version,
		current: current,
		root:    root,
		apply:   apply,
	}

	if err := p.create(); err != nil {
		return err
	}

	p.loadValues()
	p.saved = p.collect()
	p.watchChanges()
	p.updateDirty()
	p.dlg.Run()
	return nil
}

// create 构建对话框界面。
func (p *prefsDialog) create() error {
	configPath, err := config.Path()
	if err != nil {
		configPath = "（无法确定配置路径）"
	}

	if err := (decl.Dialog{
		AssignTo: &p.dlg,
		Title:    "CodeCounter 设置",
		MinSize:  decl.Size{Width: 560, Height: 350},
		Size:     decl.Size{Width: 600, Height: 420},
		Layout:   decl.VBox{MarginsZero: true},
		Children: []decl.Widget{
			decl.TabWidget{
				Pages: []decl.TabPage{
					p.generalPage(),
					p.excludePage(),
					p.displayPage(),
					p.aboutPage(),
				},
			},
			decl.GroupBox{
				Title:  "配置存储",
				Layout: decl.VBox{},
				Children: []decl.Widget{
					decl.Composite{
						Layout: decl.HBox{},
						Children: []decl.Widget{
							decl.Label{Text: "路径:"},
							decl.LineEdit{Text: configPath, ReadOnly: true, StretchFactor: 1},
							decl.PushButton{Text: "打开所在文件夹", OnClicked: p.openConfigDir},
						},
					},
				},
			},
			decl.Composite{
				Layout: decl.HBox{},
				Children: []decl.Widget{
					decl.PushButton{Text: "恢复默认", OnClicked: p.restoreDefaults},
					decl.HSpacer{},
					decl.PushButton{AssignTo: &p.applyButton, Text: "应用", OnClicked: p.applyCurrent},
					decl.PushButton{
						AssignTo:  &p.okButton,
						Text:      "确定",
						OnClicked: p.accept,
					},
					decl.PushButton{
						AssignTo:  &p.cancelButton,
						Text:      "取消",
						OnClicked: func() { p.dlg.Cancel() },
					},
				},
			},
		},
		DefaultButton: &p.okButton,
		CancelButton:  &p.cancelButton,
	}).Create(p.owner); err != nil {
		return err
	}

	p.dlg.SetFont(p.owner.Font())
	if icon := appIcon(); icon != nil {
		_ = p.dlg.SetIcon(icon)
	}
	return nil
}

// generalPage 构建「通用」标签页。
func (p *prefsDialog) generalPage() decl.TabPage {
	return decl.TabPage{
		Title:  "通用",
		Layout: decl.VBox{},
		Children: []decl.Widget{
			decl.Composite{
				Layout: decl.HBox{},
				Children: []decl.Widget{
					decl.Label{Text: "默认统计目录:"},
					decl.ComboBox{
						AssignTo:              &p.dirModeCombo,
						Model:                 dirModeLabels,
						OnCurrentIndexChanged: p.updateFixedDirEnabled,
						StretchFactor:         1,
					},
				},
			},
			decl.Composite{
				Layout: decl.HBox{},
				Children: []decl.Widget{
					decl.Label{Text: "固定目录:"},
					decl.LineEdit{AssignTo: &p.fixedDirEdit, StretchFactor: 1},
					decl.PushButton{Text: "浏览…", OnClicked: p.chooseFixedDir},
				},
			},
			decl.Label{Text: "「启动程序时的工作目录」指程序启动时的当前目录；"},
			decl.Label{Text: "从资源管理器双击启动时，通常是程序所在目录。"},
			decl.VSpacer{},
		},
	}
}

// excludePage 构建「排除规则」标签页。
func (p *prefsDialog) excludePage() decl.TabPage {
	return decl.TabPage{
		Title:  "排除规则",
		Layout: decl.VBox{},
		Children: []decl.Widget{
			decl.CheckBox{
				AssignTo:  &p.useDefaultsCheck,
				Text:      "启用内置排除名单",
				Alignment: decl.AlignHNearVCenter,
			},
			decl.CheckBox{
				AssignTo:  &p.useGitIgnoreCheck,
				Text:      "读取统计目录下的 .gitignore",
				Alignment: decl.AlignHNearVCenter,
			},
			decl.GroupBox{
				Title:  "内置排除名单",
				Layout: decl.VBox{},
				Children: []decl.Widget{
					decl.TextEdit{
						Text:     strings.Join(ignore.DefaultRules(), "、"),
						ReadOnly: true,
						VScroll:  true,
						MinSize:  decl.Size{Height: 72},
					},
				},
			},
			decl.GroupBox{
				Title:         "自定义排除规则",
				Layout:        decl.VBox{},
				StretchFactor: 1,
				Children: []decl.Widget{
					decl.Composite{
						Layout: decl.HBox{},
						Children: []decl.Widget{
							decl.PushButton{Text: "添加文件…", OnClicked: p.addExcludeFile},
							decl.PushButton{Text: "添加文件夹…", OnClicked: p.addExcludeFolder},
							decl.HSpacer{},
							decl.PushButton{Text: "清空", OnClicked: p.clearRules},
						},
					},
					decl.TextEdit{
						AssignTo:      &p.rulesEdit,
						VScroll:       true,
						HScroll:       true,
						StretchFactor: 1,
						MinSize:       decl.Size{Height: 90},
						OnTextChanged: p.refreshStaleHint,
					},
					decl.Label{AssignTo: &p.staleLabel},
					decl.Label{Text: "一行一条规则，支持 glob 通配（如 *.min.js）与 ! 取反。"},
					decl.Label{Text: "统计目录内的路径会保存为相对路径，项目整体移动后规则依然有效；"},
					decl.Label{Text: "目录外的路径保存为绝对路径，路径不存在时不会报错，但也不再生效。"},
				},
			},
		},
	}
}

// displayPage 构建「显示」标签页。
func (p *prefsDialog) displayPage() decl.TabPage {
	return decl.TabPage{
		Title:  "显示",
		Layout: decl.VBox{},
		Children: []decl.Widget{
			decl.CheckBox{
				AssignTo:  &p.showIconsCheck,
				Text:      "在条形图中显示语言图标",
				Alignment: decl.AlignHNearVCenter,
			},
			decl.CheckBox{
				AssignTo:  &p.showPercentCheck,
				Text:      "在条形图中显示代码行数占比",
				Alignment: decl.AlignHNearVCenter,
			},

			decl.Composite{
				Layout: decl.HBox{},
				Children: []decl.Widget{
					decl.Label{Text: "界面缩放:"},
					decl.ComboBox{AssignTo: &p.scaleCombo, Model: scaleLabels},
					decl.Label{Text: "（调整字体与图表密度）"},
					decl.HSpacer{},
				},
			},
			decl.VSpacer{},
		},
	}
}

// aboutPage 构建「关于」标签页。
func (p *prefsDialog) aboutPage() decl.TabPage {
	return decl.TabPage{
		Title:  "关于",
		Layout: decl.VBox{},
		Children: []decl.Widget{
			decl.Label{Text: fmt.Sprintf("CodeCounter %s", p.version)},
			decl.Label{Text: "统计项目代码行数的 Windows 原生小工具。"},
			decl.Label{Text: "项目地址: https://github.com/linfunss/codecounter"},
			decl.Label{Text: "许可证: MIT"},
			decl.VSpacer{},
		},
	}
}

// loadValues 把当前配置写入各个控件。
func (p *prefsDialog) loadValues() {
	_ = p.dirModeCombo.SetCurrentIndex(indexOfDirMode(p.current.DefaultDirMode))
	_ = p.fixedDirEdit.SetText(p.current.FixedDir)
	p.useDefaultsCheck.SetChecked(p.current.UseDefaultExcludes)
	p.useGitIgnoreCheck.SetChecked(p.current.UseGitIgnore)
	_ = p.rulesEdit.SetText(strings.Join(p.current.CustomExcludes, "\r\n"))
	p.showIconsCheck.SetChecked(p.current.ShowLanguageIcons)
	p.showPercentCheck.SetChecked(p.current.ShowPercent)
	_ = p.scaleCombo.SetCurrentIndex(indexOfScale(p.current.Scale))

	p.updateFixedDirEnabled()
	p.refreshStaleHint()
}

// collect 从各个控件收集配置。
//
// 必须在对话框关闭之前调用：walk 会在对话框关闭时释放子控件，
// 之后再读取控件只能得到零值。
func (p *prefsDialog) collect() config.Config {
	cfg := p.current

	idx := p.dirModeCombo.CurrentIndex()
	if idx < 0 || idx >= len(dirModeValues) {
		idx = 0
	}
	cfg.DefaultDirMode = dirModeValues[idx]
	cfg.FixedDir = strings.TrimSpace(p.fixedDirEdit.Text())

	cfg.UseDefaultExcludes = p.useDefaultsCheck.Checked()
	cfg.UseGitIgnore = p.useGitIgnoreCheck.Checked()
	cfg.CustomExcludes = splitRuleLines(p.rulesEdit.Text())

	cfg.ShowLanguageIcons = p.showIconsCheck.Checked()
	cfg.ShowPercent = p.showPercentCheck.Checked()
	cfg.MaxBars = 0 // 图表通过滚动展示全部语言。
	if scale := p.scaleCombo.CurrentIndex(); scale >= 0 && scale < len(scaleValues) {
		cfg.Scale = scaleValues[scale]
	}

	cfg.Normalize()

	return cfg
}

// applyCurrent 收集并提交当前设置，供「应用」按钮使用。
func (p *prefsDialog) applyCurrent() {
	cfg := p.collect()
	if err := p.apply(cfg); err != nil {
		walk.MsgBox(p.dlg, "保存设置失败", err.Error(), walk.MsgBoxIconError)
		return
	}

	p.current = cfg
	p.saved = cfg
	p.dlg.SetFont(p.owner.Font())
	p.updateDirty()
	p.refreshStaleHint()
}

// accept 先提交设置再关闭对话框。
func (p *prefsDialog) accept() {
	cfg := p.collect()
	if err := p.apply(cfg); err != nil {
		walk.MsgBox(p.dlg, "保存设置失败", err.Error(), walk.MsgBoxIconError)
		return
	}

	p.current = cfg
	p.dlg.Accept()
}

// updateFixedDirEnabled 仅在选择「固定目录」时启用固定目录输入框。
func (p *prefsDialog) updateFixedDirEnabled() {
	if p.fixedDirEdit == nil {
		return
	}
	p.fixedDirEdit.SetEnabled(p.selectedDirMode() == config.DirModeFixed)
}

// selectedDirMode 返回当前选中的默认目录策略。
func (p *prefsDialog) selectedDirMode() config.DirMode {
	idx := p.dirModeCombo.CurrentIndex()
	if idx < 0 || idx >= len(dirModeValues) {
		return config.DirModeWorkingDir
	}
	return dirModeValues[idx]
}

// chooseFixedDir 让用户选择固定统计目录。
func (p *prefsDialog) chooseFixedDir() {
	dir, ok := chooseFolder(p.dlg, "选择固定统计目录", p.fixedDirEdit.Text())
	if !ok || dir == "" {
		return
	}
	_ = p.fixedDirEdit.SetText(dir)
}

// addExcludeFile 选择要忽略的文件并追加为规则。
func (p *prefsDialog) addExcludeFile() {
	dlg := walk.FileDialog{
		Title:          "选择要忽略的文件",
		InitialDirPath: p.root,
		Filter:         "所有文件 (*.*)|*.*",
	}

	accepted, err := dlg.ShowOpenMultiple(p.dlg)
	if err != nil || !accepted {
		return
	}
	for _, path := range dlg.FilePaths {
		p.appendRule(p.ruleForPath(path))
	}
}

// addExcludeFolder 选择要忽略的文件夹并追加为规则。
func (p *prefsDialog) addExcludeFolder() {
	dir, ok := chooseFolder(p.dlg, "选择要忽略的文件夹", p.root)
	if !ok || dir == "" {
		return
	}
	p.appendRule(p.ruleForPath(dir))
}

// ruleForPath 把用户选择的路径转换为排除规则。
//
// 统计目录内的路径写成相对路径，这样项目整体移动后规则依然有效；
// 目录之外的路径只能写成绝对路径。
func (p *prefsDialog) ruleForPath(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		return filepath.ToSlash(path)
	}

	root := p.root
	if root == "" {
		root, _ = os.Getwd()
	}
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return filepath.ToSlash(abs)
	}

	rel, err := filepath.Rel(rootAbs, abs)
	if err == nil && rel != "." && !strings.HasPrefix(rel, "..") {
		return filepath.ToSlash(rel)
	}
	return filepath.ToSlash(abs)
}

// appendRule 把规则追加到自定义规则框，已存在的规则会被忽略。
func (p *prefsDialog) appendRule(rule string) {
	rule = strings.TrimSpace(rule)
	if rule == "" {
		return
	}

	rules := splitRuleLines(p.rulesEdit.Text())
	for _, existing := range rules {
		if existing == rule {
			return
		}
	}

	rules = append(rules, rule)
	_ = p.rulesEdit.SetText(strings.Join(rules, "\r\n"))
	p.refreshStaleHint()
}

// clearRules 清空全部自定义规则。
func (p *prefsDialog) clearRules() {
	if len(splitRuleLines(p.rulesEdit.Text())) == 0 {
		return
	}

	const prompt = "确定要清空全部自定义排除规则吗？"
	if walk.MsgBox(p.dlg, "清空自定义规则", prompt, walk.MsgBoxYesNo|walk.MsgBoxIconQuestion) != walk.DlgCmdYes {
		return
	}

	_ = p.rulesEdit.SetText("")
	p.refreshStaleHint()
}

// refreshStaleHint 提示哪些绝对路径规则已经失效。
//
// 规则本身只是文本匹配，路径失效不会导致统计出错，但用户不容易察觉，
// 因此在这里显式提示，便于清理。
func (p *prefsDialog) refreshStaleHint() {
	if p.staleLabel == nil {
		return
	}

	stale := 0
	for _, rule := range splitRuleLines(p.rulesEdit.Text()) {
		if !isAbsolutePathRule(rule) {
			continue
		}
		if _, err := os.Stat(rule); err != nil {
			stale++
		}
	}

	if stale == 0 {
		_ = p.staleLabel.SetText("")
		return
	}
	_ = p.staleLabel.SetText(fmt.Sprintf("提示：有 %d 条规则指向的路径已不存在，它们不会再生效，可以删除。", stale))
}

// isAbsolutePathRule 判断规则是否为绝对路径，而不是名称或通配模式。
func isAbsolutePathRule(rule string) bool {
	rule = strings.TrimPrefix(strings.TrimSpace(rule), "!")
	if strings.ContainsAny(rule, "*?") {
		return false
	}
	return filepath.IsAbs(rule)
}

// restoreDefaults 把界面上的设置项恢复为默认值。
func (p *prefsDialog) restoreDefaults() {
	const prompt = "确定要把所有设置恢复为默认值吗？"

	if walk.MsgBox(p.dlg, "恢复默认设置", prompt, walk.MsgBoxYesNo|walk.MsgBoxIconQuestion) != walk.DlgCmdYes {
		return
	}

	last := p.current.LastDir
	p.current = config.Defaults()
	p.current.LastDir = last
	p.loadValues()
	p.updateDirty()
}

// openConfigDir 在资源管理器中打开配置目录。
func (p *prefsDialog) openConfigDir() {
	dir, err := config.Dir()
	if err != nil {
		walk.MsgBox(p.dlg, "打开失败", err.Error(), walk.MsgBoxIconError)
		return
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		walk.MsgBox(p.dlg, "打开失败", err.Error(), walk.MsgBoxIconError)
		return
	}
	if err := exec.Command("explorer.exe", dir).Start(); err != nil {
		walk.MsgBox(p.dlg, "打开失败", err.Error(), walk.MsgBoxIconError)
	}
}

// indexOfDirMode 返回目录策略在下拉框中的下标。
func indexOfDirMode(mode config.DirMode) int {
	for i, v := range dirModeValues {
		if v == mode {
			return i
		}
	}
	return 0
}

// indexOfMaxBars 返回数量上限在下拉框中的下标，未知取值回退到「20」。
func indexOfMaxBars(value int) int {
	for i, v := range maxBarsValues {
		if v == value {
			return i
		}
	}
	return 1
}

// indexOfScale 返回缩放系数在下拉框中的下标，未知取值回退到「100%」。
func indexOfScale(value float64) int {
	for i, v := range scaleValues {
		if v == value {
			return i
		}
	}
	return 2
}

// splitRuleLines 把多行文本拆分为规则列表，忽略空行。
func splitRuleLines(text string) []string {
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")

	rules := make([]string, 0, len(lines))
	for _, line := range lines {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			rules = append(rules, trimmed)
		}
	}
	return rules
}

func (p *prefsDialog) updateDirty() {
	p.applyButton.SetEnabled(!reflect.DeepEqual(p.collect(), p.saved))
}
func (p *prefsDialog) watchChanges() {
	for _, c := range []*walk.ComboBox{p.dirModeCombo, p.scaleCombo} {
		c.CurrentIndexChanged().Attach(p.updateDirty)
	}
	for _, c := range []*walk.CheckBox{p.useDefaultsCheck, p.useGitIgnoreCheck, p.showIconsCheck, p.showPercentCheck} {
		c.CheckedChanged().Attach(p.updateDirty)
	}
	p.fixedDirEdit.TextChanged().Attach(p.updateDirty)
	p.rulesEdit.TextChanged().Attach(p.updateDirty)
}
