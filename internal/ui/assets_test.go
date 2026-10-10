package ui

import (
	"bytes"
	"image/png"
	"io/fs"
	"testing"
)

// TestLanguageLogoFilesExist 验证语言与图标的映射和内嵌资源一一对应。
func TestLanguageLogoFilesExist(t *testing.T) {
	if len(languageLogoFiles) == 0 {
		t.Fatal("语言图标映射不应为空")
	}

	for language, file := range languageLogoFiles {
		data, err := iconAssets.ReadFile("assets/icons/" + file)
		if err != nil {
			t.Errorf("%s 对应的图标文件缺失: %v", language, err)
			continue
		}
		if _, err := png.Decode(bytes.NewReader(data)); err != nil {
			t.Errorf("%s 的图标不是有效的 PNG: %v", language, err)
		}
	}
}

// TestIconAssetsHaveNoOrphan 验证内嵌图标都有对应的语言映射，避免残留文件。
func TestIconAssetsHaveNoOrphan(t *testing.T) {
	entries, err := fs.ReadDir(iconAssets, "assets/icons")
	if err != nil {
		t.Fatalf("读取内嵌图标目录失败: %v", err)
	}

	used := make(map[string]bool, len(languageLogoFiles))
	for _, file := range languageLogoFiles {
		used[file] = true
	}

	for _, entry := range entries {
		if !used[entry.Name()] {
			t.Errorf("图标 %s 没有任何语言引用", entry.Name())
		}
	}
}

// TestLanguageLogoUnknown 验证未收录语言返回 nil，以便回退到自绘徽章。
func TestLanguageLogoUnknown(t *testing.T) {
	if logo := languageLogo("不存在的语言"); logo != nil {
		t.Error("未收录的语言应返回 nil")
	}
}
