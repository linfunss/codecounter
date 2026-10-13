package ui

import (
	"bytes"
	"embed"
	"image/png"
	"sync"

	"github.com/lxn/walk"
)

// iconAssets 内嵌全部语言图标 PNG。
//
// 图标取自 simple-icons 项目（CC0-1.0），由 tools/genicons 生成，
// 相关说明见仓库根目录的 NOTICE。
//
//go:embed assets/icons/*.png
var iconAssets embed.FS

// iconDPI 是内嵌图标位图的基准 DPI。
// 图标始终按目标物理像素尺寸绘制，这里只影响位图自身的 DPI 元数据。
const iconDPI = 96

// appIconPNG 是应用图标的 PNG 版本，用于设置窗口与任务栏图标。
// 可执行文件本身的图标由 winres/app.ico 通过资源文件嵌入。
//
//go:embed assets/icon.png
var appIconPNG []byte

// appLogoPNG 是「关于」页展示的完整 Logo 透明 PNG。
// lxn/walk 基于 Win32/GDI，不能直接渲染 SVG，因此运行时使用 PNG 版本。
//
//go:embed assets/logo.png
var appLogoPNG []byte

// languageLogoFiles 把 scc 输出的语言名映射到内置图标文件名。
// 未收录的语言会在界面上回退为自绘的品牌色徽章。
var languageLogoFiles = map[string]string{
	"C":          "c.png",
	"C#":         "csharp.png",
	"C++":        "cplusplus.png",
	"CMake":      "cmake.png",
	"CSS":        "css.png",
	"Dart":       "dart.png",
	"Dockerfile": "dockerfile.png",
	"Go":         "go.png",
	"Gradle":     "gradle.png",
	"HTML":       "html.png",
	"JSON":       "json.png",
	"Java":       "java.png",
	"JavaScript": "javascript.png",
	"Kotlin":     "kotlin.png",
	"Lua":        "lua.png",
	"Markdown":   "markdown.png",
	"PHP":        "php.png",
	"Perl":       "perl.png",
	"PowerShell": "powershell.png",
	"Python":     "python.png",
	"R":          "r.png",
	"Ruby":       "ruby.png",
	"Rust":       "rust.png",
	"SCSS":       "scss.png",
	"Scala":      "scala.png",
	"Shell":      "shell.png",
	"Svelte":     "svelte.png",
	"Swift":      "swift.png",
	"Terraform":  "terraform.png",
	"TypeScript": "typescript.png",
	"Vue":        "vue.png",
	"YAML":       "yaml.png",
}

// logoCache 缓存已解码的图标位图，避免每次重绘都重新解码。
// 访问由 logoMu 保护，保证绘制线程与测试并发安全。
var (
	logoMu    sync.Mutex
	logoCache = map[string]*walk.Bitmap{}
)

var (
	appIconOnce sync.Once
	appIconVal  *walk.Icon

	appLogoOnce sync.Once
	appLogoVal  *walk.Bitmap
)

// appIcon 返回应用图标，加载失败时返回 nil。
func appIcon() *walk.Icon {
	appIconOnce.Do(func() {
		img, err := png.Decode(bytes.NewReader(appIconPNG))
		if err != nil {
			return
		}
		icon, err := walk.NewIconFromImageForDPI(img, iconDPI)
		if err != nil {
			return
		}
		appIconVal = icon
	})
	return appIconVal
}

// appLogo 返回「关于」页使用的完整 Logo 位图，加载失败时返回 nil。
func appLogo() *walk.Bitmap {
	appLogoOnce.Do(func() {
		img, err := png.Decode(bytes.NewReader(appLogoPNG))
		if err != nil {
			return
		}
		bmp, err := walk.NewBitmapFromImageForDPI(img, iconDPI)
		if err != nil {
			return
		}
		appLogoVal = bmp
	})
	return appLogoVal
}

// languageLogo 返回语言对应的图标位图。
//
// 未收录该语言或资源加载失败时返回 nil，调用方应回退到自绘徽章。
func languageLogo(language string) *walk.Bitmap {
	file, ok := languageLogoFiles[language]
	if !ok {
		return nil
	}

	logoMu.Lock()
	defer logoMu.Unlock()

	if bmp, cached := logoCache[language]; cached {
		return bmp
	}

	data, err := iconAssets.ReadFile("assets/icons/" + file)
	if err != nil {
		return nil
	}

	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return nil
	}

	bmp, err := walk.NewBitmapFromImageForDPI(img, iconDPI)
	if err != nil {
		return nil
	}

	logoCache[language] = bmp
	return bmp
}
