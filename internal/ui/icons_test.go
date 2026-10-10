package ui

import (
	"testing"

	"github.com/lxn/walk"
)

// TestBadgeText 验证图标短标识的取法。
func TestBadgeText(t *testing.T) {
	cases := map[string]string{
		"JavaScript":  "JS",
		"TypeScript":  "TS",
		"Python":      "Py",
		"Rust":        "Rs",
		"Dockerfile":  "Dk",
		"Go":          "G",
		"UnknownLang": "U",
		"":            "?",
	}

	for in, want := range cases {
		if got := badgeText(in); got != want {
			t.Errorf("badgeText(%q) = %q, 期望 %q", in, got, want)
		}
	}
}

// TestLanguageBadgeStable 验证同一语言的颜色与标识保持稳定。
func TestLanguageBadgeStable(t *testing.T) {
	first := languageBadge("SomeUnknownLanguage")
	second := languageBadge("SomeUnknownLanguage")

	if first != second {
		t.Errorf("同一语言应得到相同图标参数: %+v 与 %+v", first, second)
	}
	if first.text == "" {
		t.Error("图标文字不应为空")
	}
}

// TestLanguageBadgeKnownColor 验证已收录语言使用品牌色。
func TestLanguageBadgeKnownColor(t *testing.T) {
	if got := languageBadge("Go"); got.color != brandColors["Go"] {
		t.Errorf("Go 应使用品牌色，实际得到 %v", got.color)
	}
}

// TestContrastColor 验证前景色随背景亮度切换。
func TestContrastColor(t *testing.T) {
	if got := contrastColor(walk.RGB(0x00, 0x00, 0x00)); got != walk.RGB(0xFF, 0xFF, 0xFF) {
		t.Errorf("深色背景应使用白色文字，实际得到 %v", got)
	}
	if got := contrastColor(walk.RGB(0xFF, 0xFF, 0xFF)); got != walk.RGB(0x20, 0x20, 0x20) {
		t.Errorf("浅色背景应使用深色文字，实际得到 %v", got)
	}
}
