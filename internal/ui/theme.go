package ui

import (
	"math"
	"strings"
	"sync"
	"unsafe"

	"github.com/lxn/walk"
	"github.com/lxn/win"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

// 界面统一内边距（96 DPI 逻辑像素）。
//
// 主窗口与设置对话框共用同一组数值，让两类窗口的内容左边距一致；
// 数值比 walk 默认的 9 稍大，给内容与窗口边框之间留出呼吸空间。
const (
	// uiMarginH 是顶层内容区与窗口边框之间的左右内边距。
	uiMarginH = 14
	// uiMarginV 是顶层内容区与窗口边框之间的上下内边距。
	uiMarginV = 8
	// uiTabMargin 是设置标签页内容与标签框之间的内边距，
	// 取值比顶层略小，因为标签框本身已经带了一圈留白。
	uiTabMargin = 12
)

// uiBasePointSize 是缩放系数为 1.0 时的界面字号（磅）。
//
// 9pt 与简体中文版 Windows 10 的默认 UI 字号一致，比 walk 内置的
// 「MS Shell Dlg 2, 8pt」更大，在高分屏上更容易阅读。
const uiBasePointSize = 9

// uiFontCandidates 是按优先级排列的界面字体候选。
//
// walk 默认使用「MS Shell Dlg 2」，它由系统映射到 Microsoft Sans Serif。
// 该字体没有汉字字形，GDI 会把汉字交给字体链接的回退字体（小字号下是点阵
// 宋体，笔画很细），而数字仍由西文字体绘制（笔画偏粗），于是在高分屏上出现
// 「中文细、数字粗」的割裂观感。改用同时覆盖中英文的字体即可让两者一致。
var uiFontCandidates = []string{
	"Microsoft YaHei UI",
	"Microsoft YaHei",
	"Segoe UI",
	"MS Shell Dlg 2",
}

// fontsRegistryPath 是 Windows 记录已安装字体名的注册表位置。
const fontsRegistryPath = `SOFTWARE\Microsoft\Windows NT\CurrentVersion\Fonts`

var (
	uiFontFamilyOnce sync.Once
	uiFontFamilyName string
)

// uiFontFamily 返回当前系统上可用的界面字体族名。
//
// 所有候选都不可用时回退到「MS Shell Dlg 2」，由系统决定实际字体。
func uiFontFamily() string {
	uiFontFamilyOnce.Do(func() {
		installed := installedFontFamilies()
		for _, candidate := range uiFontCandidates {
			if _, ok := installed[strings.ToLower(candidate)]; ok {
				uiFontFamilyName = candidate
				return
			}
		}

		uiFontFamilyName = uiFontCandidates[len(uiFontCandidates)-1]
	})

	return uiFontFamilyName
}

// installedFontFamilies 返回本机已安装的字体族名，键统一为小写。
func installedFontFamilies() map[string]struct{} {
	families := make(map[string]struct{})

	for _, root := range []registry.Key{registry.LOCAL_MACHINE, registry.CURRENT_USER} {
		key, err := registry.OpenKey(root, fontsRegistryPath, registry.QUERY_VALUE)
		if err != nil {
			continue
		}

		names, err := key.ReadValueNames(-1)
		// 读表失败说明该根键没有可用信息，继续看下一个根键
		_ = key.Close()
		if err != nil {
			continue
		}

		for _, name := range names {
			families[strings.ToLower(fontFamilyOfEntry(name))] = struct{}{}
		}
	}

	return families
}

// fontFamilyOfEntry 把「Microsoft YaHei UI (TrueType)」这类注册表值名还原成字体族名。
func fontFamilyOfEntry(name string) string {
	name = strings.TrimSpace(name)
	if idx := strings.LastIndex(name, " ("); idx > 0 && strings.HasSuffix(name, ")") {
		name = strings.TrimSpace(name[:idx])
	}
	return name
}

// newUIFont 按缩放系数创建界面字体，失败时返回 nil。
func newUIFont(scale float64) *walk.Font {
	font, err := walk.NewFont(uiFontFamily(), scaledPointSize(scale), 0)
	if err != nil {
		return nil
	}
	return font
}

// scaledPointSize 把界面缩放系数换算成字号（磅）。
//
// 字号本身与 DPI 无关：walk 会按窗口所在显示器的 DPI 把磅值换算成像素高度，
// 因此这里不能再叠加系统缩放，否则高分屏上会出现二次放大。
func scaledPointSize(scale float64) int {
	if scale <= 0 {
		scale = 1
	}

	size := int(math.Round(float64(uiBasePointSize) * scale))
	if size < 6 {
		size = 6
	}
	return size
}

// Windows 11 会给顶层窗口加上圆角与彩色边框，Windows 10 则是直角窗框。
// 以下常量用于把窗口外观切回 Windows 10 风格。
const (
	dwmwaWindowCornerPreference = 33
	dwmcpDoNotRound             = 1
)

var procDwmSetWindowAttribute = windows.NewLazySystemDLL("dwmapi.dll").NewProc("DwmSetWindowAttribute")

// useWindows10WindowFrame 关闭 Windows 11 的圆角窗框，恢复 Windows 10 的直角外观。
//
// 该属性只有 Windows 11 识别，Windows 10 调用会返回 E_INVALIDARG；
// 调用失败说明系统不支持，保持系统默认外观即可。
func useWindows10WindowFrame(handle win.HWND) {
	if handle == 0 {
		return
	}
	if err := procDwmSetWindowAttribute.Find(); err != nil {
		return
	}

	preference := int32(dwmcpDoNotRound)
	// 调用失败没有补救手段，忽略返回值
	_, _, _ = procDwmSetWindowAttribute.Call(
		uintptr(handle),
		uintptr(dwmwaWindowCornerPreference),
		uintptr(unsafe.Pointer(&preference)),
		unsafe.Sizeof(preference),
	)
}
