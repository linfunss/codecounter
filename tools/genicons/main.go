// genicons 从 simple-icons 下载编程语言图标，光栅化为 PNG 供界面嵌入。
//
// 该工具位于独立子模块中，不参与主模块构建，也不引入运行时依赖。
// 图标来源：simple-icons（CC0-1.0），许可证与商标说明见仓库根目录的 NOTICE。
//
// 用法：
//
//	go run . -out ../../internal/ui/assets/icons
package main

import (
	"bytes"
	"flag"
	"fmt"
	"image"
	"image/png"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/srwiley/oksvg"
	"github.com/srwiley/rasterx"
)

// iconSpec 描述一个语言图标。
type iconSpec struct {
	// Language 是 scc 输出的语言名，也是生成的文件名。
	Language string
	// Slug 是 simple-icons 中的图标标识。
	Slug string
	// Color 是品牌色的十六进制表示（不含 #）。
	Color string
}

// icons 是需要生成图标的语言清单。
// 未收录的语言在界面上会回退为自绘的品牌色徽章。
var icons = []iconSpec{
	{"Go", "go", "00ADD8"},
	{"JavaScript", "javascript", "F7DF1E"},
	{"TypeScript", "typescript", "3178C6"},
	{"Python", "python", "3776AB"},
	{"Java", "openjdk", "E76F00"},
	{"C", "c", "A8B9CC"},
	{"C++", "cplusplus", "00599C"},
	{"C#", "csharp", "512BD4"},
	{"Rust", "rust", "000000"},
	{"PHP", "php", "777BB4"},
	{"Ruby", "ruby", "CC342D"},
	{"Kotlin", "kotlin", "7F52FF"},
	{"Swift", "swift", "F05138"},
	{"Scala", "scala", "DC322F"},
	{"Dart", "dart", "0175C2"},
	{"Lua", "lua", "2C2D72"},
	{"Perl", "perl", "39457E"},
	{"R", "r", "276DC3"},
	{"Shell", "gnubash", "4EAA25"},
	{"PowerShell", "powershell", "5391FE"},
	{"HTML", "html5", "E34F26"},
	{"CSS", "css", "663399"},
	{"SCSS", "sass", "CC6699"},
	{"Vue", "vuedotjs", "4FC08D"},
	{"Svelte", "svelte", "FF3E00"},
	{"JSON", "json", "000000"},
	{"YAML", "yaml", "CB171E"},
	{"Markdown", "markdown", "000000"},
	{"CMake", "cmake", "064F7C"},
	{"Dockerfile", "docker", "2496ED"},
	{"Terraform", "terraform", "7B42BC"},
	{"Gradle", "gradle", "02303A"},
}

// svgSizeAttrPattern 匹配 SVG 中的 width/height 属性。
var svgSizeAttrPattern = regexp.MustCompile(`\s(?:width|height)="[^"]*"`)

// size 是生成 PNG 的边长（像素）。
// 界面图标为 20 逻辑像素，64 像素源图在高 DPI 下缩小绘制仍然清晰。
const size = 64

func main() {
	outDir := flag.String("out", "../../internal/ui/assets/icons", "PNG 输出目录")
	flag.Parse()

	if err := os.MkdirAll(*outDir, 0o755); err != nil {
		log.Fatalf("创建输出目录失败: %v", err)
	}

	client := &http.Client{Timeout: 30 * time.Second}

	var ok, failed int
	for _, spec := range icons {
		img, err := renderIcon(client, spec)
		if err != nil {
			log.Printf("跳过 %s: %v", spec.Language, err)
			failed++
			continue
		}

		path := filepath.Join(*outDir, fileName(spec.Language))
		if err := writePNG(path, img); err != nil {
			log.Fatalf("写入 %s 失败: %v", path, err)
		}
		fmt.Printf("生成 %s\n", path)
		ok++
	}

	fmt.Printf("\n完成：成功 %d 个，跳过 %d 个\n", ok, failed)
}

// renderIcon 下载并光栅化单个图标。
func renderIcon(client *http.Client, spec iconSpec) (image.Image, error) {
	url := fmt.Sprintf("https://api.iconify.design/simple-icons/%s.svg", spec.Slug)

	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("下载失败，HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// simple-icons 使用 currentColor，替换为品牌色后即可直接光栅化
	svg := strings.ReplaceAll(string(body), "currentColor", "#"+spec.Color)

	// Iconify 输出的 width/height 为 "1em"，会让 oksvg 解析出零尺寸 viewBox，
	// 因此去掉这两个属性，只保留 viewBox 作为尺寸依据
	svg = svgSizeAttrPattern.ReplaceAllString(svg, "")

	return rasterize(svg)
}

// rasterize 把 SVG 渲染为指定尺寸的位图。
func rasterize(svg string) (image.Image, error) {
	icon, err := oksvg.ReadIconStream(strings.NewReader(svg))
	if err != nil {
		return nil, fmt.Errorf("解析 SVG 失败: %w", err)
	}

	img := image.NewRGBA(image.Rect(0, 0, size, size))
	scanner := rasterx.NewScannerGV(size, size, img, img.Bounds())
	dasher := rasterx.NewDasher(size, size, scanner)

	icon.SetTarget(0, 0, float64(size), float64(size))
	icon.Draw(dasher, 1.0)

	return img, nil
}

// writePNG 把图像写入文件。
func writePNG(path string, img image.Image) error {
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return err
	}
	return os.WriteFile(path, buf.Bytes(), 0o644)
}

// fileName 把语言名转换为安全的文件名。
func fileName(language string) string {
	replacer := strings.NewReplacer(
		"#", "sharp",
		"+", "plus",
		" ", "-",
		"/", "-",
	)
	return strings.ToLower(replacer.Replace(language)) + ".png"
}
