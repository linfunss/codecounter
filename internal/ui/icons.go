package ui

import (
	"strings"

	"github.com/lxn/walk"
)

// badge 描述语言图标的绘制参数。
type badge struct {
	// text 是图标中显示的短标识，最多两个字符。
	text string
	// color 是图标背景色。
	color walk.Color
}

// brandColors 保存常见编程语言的品牌色，键为 scc 输出的语言名。
// 未收录的语言会从 badgePalette 中按名称哈希选择颜色，保证同一语言颜色稳定。
var brandColors = map[string]walk.Color{
	"C":                walk.RGB(0x55, 0x55, 0x55),
	"C Header":         walk.RGB(0x6B, 0x6B, 0x6B),
	"C#":               walk.RGB(0x68, 0x21, 0x7A),
	"C++":              walk.RGB(0x00, 0x59, 0x9C),
	"CMake":            walk.RGB(0x06, 0x4F, 0x7C),
	"CSS":              walk.RGB(0x56, 0x3D, 0x7C),
	"Dart":             walk.RGB(0x01, 0x75, 0x9B),
	"Dockerfile":       walk.RGB(0x24, 0x96, 0xED),
	"Go":               walk.RGB(0x00, 0xAD, 0xD8),
	"Gradle":           walk.RGB(0x02, 0x36, 0x44),
	"HTML":             walk.RGB(0xE3, 0x4C, 0x26),
	"Java":             walk.RGB(0xE7, 0x6F, 0x00),
	"JavaScript":       walk.RGB(0xB8, 0xA0, 0x00),
	"JSON":             walk.RGB(0x29, 0x2A, 0x33),
	"Kotlin":           walk.RGB(0x7F, 0x52, 0xFF),
	"Less":             walk.RGB(0x1D, 0x36, 0x5D),
	"Lua":              walk.RGB(0x2C, 0x2C, 0x72),
	"Makefile":         walk.RGB(0x42, 0x68, 0x5B),
	"Markdown":         walk.RGB(0x08, 0x31, 0x4A),
	"MATLAB":           walk.RGB(0xE1, 0x67, 0x37),
	"Objective-C":      walk.RGB(0x43, 0x82, 0xC8),
	"Perl":             walk.RGB(0x39, 0x43, 0x57),
	"PHP":              walk.RGB(0x62, 0x62, 0xA5),
	"PowerShell":       walk.RGB(0x01, 0x24, 0x56),
	"Protocol Buffers": walk.RGB(0x42, 0x85, 0xA4),
	"Python":           walk.RGB(0x37, 0x76, 0xAB),
	"R":                walk.RGB(0x27, 0x6D, 0xC2),
	"Ruby":             walk.RGB(0xCC, 0x34, 0x2D),
	"Rust":             walk.RGB(0x9B, 0x63, 0x3C),
	"SCSS":             walk.RGB(0xCD, 0x67, 0x99),
	"SQL":              walk.RGB(0x00, 0x66, 0x99),
	"Scala":            walk.RGB(0xDC, 0x32, 0x2D),
	"Shell":            walk.RGB(0x4E, 0xAA, 0x25),
	"Svelte":           walk.RGB(0xFF, 0x3E, 0x00),
	"Swift":            walk.RGB(0xFA, 0x73, 0x42),
	"Terraform":        walk.RGB(0x7B, 0x42, 0xBC),
	"TypeScript":       walk.RGB(0x31, 0x78, 0xC6),
	"Vue":              walk.RGB(0x41, 0xB8, 0x83),
	"XML":              walk.RGB(0x00, 0x66, 0x99),
	"YAML":             walk.RGB(0xCB, 0x17, 0x1E),
	"License":          walk.RGB(0x78, 0x78, 0x78),
	"Plain Text":       walk.RGB(0x8A, 0x8A, 0x8A),
}

// abbreviations 保存语言的惯用缩写，用于图标中的短标识。
var abbreviations = map[string]string{
	"JavaScript":       "JS",
	"TypeScript":       "TS",
	"Python":           "Py",
	"Kotlin":           "Kt",
	"Rust":             "Rs",
	"Ruby":             "Rb",
	"Swift":            "Sw",
	"Dart":             "Dt",
	"Lua":              "Lu",
	"Shell":            "Sh",
	"PowerShell":       "PS",
	"Markdown":         "Md",
	"Makefile":         "Mk",
	"CMake":            "CM",
	"Terraform":        "Tf",
	"Dockerfile":       "Dk",
	"Protocol Buffers": "Pb",
}

// badgePalette 是回退调色板，用于未收录品牌色的语言。
var badgePalette = []walk.Color{
	walk.RGB(0x3B, 0x7D, 0xD8),
	walk.RGB(0x2E, 0xA0, 0x6B),
	walk.RGB(0xD1, 0x6B, 0x2A),
	walk.RGB(0x8B, 0x4F, 0xD1),
	walk.RGB(0xC0, 0x39, 0x4B),
	walk.RGB(0x2A, 0x8C, 0x9E),
	walk.RGB(0x7A, 0x6A, 0x2F),
	walk.RGB(0x5A, 0x5F, 0x6B),
}

// languageBadge 返回语言对应的图标参数。
func languageBadge(language string) badge {
	color, ok := brandColors[language]
	if !ok {
		color = badgePalette[hashString(language)%uint32(len(badgePalette))]
	}
	return badge{text: badgeText(language), color: color}
}

// badgeText 返回图标中显示的短标识。
func badgeText(language string) string {
	if s, ok := abbreviations[language]; ok {
		return s
	}
	for _, r := range language {
		if r == ' ' {
			continue
		}
		return strings.ToUpper(string(r))
	}
	return "?"
}

// contrastColor 依据背景亮度返回可读的前景色。
func contrastColor(background walk.Color) walk.Color {
	r := float64(byte(background & 0xFF))
	g := float64(byte((background >> 8) & 0xFF))
	b := float64(byte((background >> 16) & 0xFF))

	if 0.299*r+0.587*g+0.114*b > 150 {
		return walk.RGB(0x20, 0x20, 0x20)
	}
	return walk.RGB(0xFF, 0xFF, 0xFF)
}

// hashString 计算字符串的稳定哈希，用于为未知语言分配固定颜色。
func hashString(s string) uint32 {
	var h uint32 = 2166136261
	for _, r := range s {
		h ^= uint32(r)
		h *= 16777619
	}
	return h
}
