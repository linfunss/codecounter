package ui

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/lxn/walk"
	decl "github.com/lxn/walk/declarative"

	"github.com/linfunss/codecounter/internal/config"
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

// prefsDialog 是设置对话框，负责展示配置项并在确认后回传新配置。
type prefsDialog struct {
	owner   walk.Form
	version string
	current config.Config

	dlg *walk.Dialog

	dirModeCombo *walk.ComboBox
	fixedDirEdit *walk.LineEdit

	useDefaultsCheck  *walk.CheckBox
	useGitIgnoreCheck *walk.CheckBox
	rulesEdit         *walk.TextEdit

	showIconsCheck *walk.CheckBox
	maxBarsCombo   *walk.ComboBox

	okButton     *walk.PushButton
	cancelButton *walk.PushButton
}

// showPrefs 打开设置对话框。
//
// 返回更新后的配置以及用户是否点击了确定。
func showPrefs(owner walk.Form, version string, current config.Config) (config.Config, bool, error) {
	p := &prefsDialog{owner: owner, version: version, current: current}

	if err := p.create(); err != nil {
		return current, false, err
	}

	p.loadValues()

	if p.dlg.Run() != walk.DlgCmdOK {
		return current, false, nil
	}
	return p.collect(), true, nil
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
		MinSize:  decl.Size{Width: 660, Height: 540},
		Size:     decl.Size{Width: 680, Height: 560},
		Layout:   decl.VBox{},
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
					decl.PushButton{AssignTo: &p.okButton, Text: "确定", OnClicked: func() { p.dlg.Accept() }},
					decl.PushButton{AssignTo: &p.cancelButton, Text: "取消", OnClicked: func() { p.dlg.Cancel() }},
				},
			},
		},
		DefaultButton: &p.okButton,
		CancelButton:  &p.cancelButton,
	}).Create(p.owner); err != nil {
		return err
	}

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
			decl.Label{Text: "提示:「启动程序时的工作目录」指程序启动时的当前目录；"},
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
				Text:      "启用内置排除名单（依赖、缓存、构建产物与编辑器目录）",
				Alignment: decl.AlignHNearVCenter,
			},
			decl.CheckBox{
				AssignTo:  &p.useGitIgnoreCheck,
				Text:      "读取统计目录下的 .gitignore",
				Alignment: decl.AlignHNearVCenter,
			},
			decl.GroupBox{
				Title:         "自定义排除规则",
				Layout:        decl.VBox{},
				StretchFactor: 1,
				Children: []decl.Widget{
					decl.TextEdit{
						AssignTo:      &p.rulesEdit,
						VScroll:       true,
						HScroll:       true,
						StretchFactor: 1,
						MinSize:       decl.Size{Height: 150},
					},
					decl.Label{Text: "一行一条规则，支持 glob 通配（如 *.min.js）；"},
					decl.Label{Text: "以 ! 开头表示重新纳入统计，以 # 开头的行视为注释。"},
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
			decl.Composite{
				Layout: decl.HBox{},
				Children: []decl.Widget{
					decl.Label{Text: "条形图最多显示:"},
					decl.ComboBox{AssignTo: &p.maxBarsCombo, Model: maxBarsLabels},
					decl.Label{Text: "种语言"},
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
	_ = p.maxBarsCombo.SetCurrentIndex(indexOfMaxBars(p.current.MaxBars))

	p.updateFixedDirEnabled()
}

// collect 从各个控件收集配置。
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
	if bars := p.maxBarsCombo.CurrentIndex(); bars >= 0 && bars < len(maxBarsValues) {
		cfg.MaxBars = maxBarsValues[bars]
	}

	cfg.Normalize()
	return cfg
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
	dlg := walk.FileDialog{
		Title:          "选择固定统计目录",
		InitialDirPath: p.fixedDirEdit.Text(),
	}

	if ok, err := dlg.ShowBrowseFolder(p.dlg); err == nil && ok && dlg.FilePath != "" {
		_ = p.fixedDirEdit.SetText(dlg.FilePath)
	}
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
