package ui

import "testing"

// TestScaledPointSize 验证字号换算：以 uiBasePointSize 为基准按缩放系数取整，并带下限保护。
func TestScaledPointSize(t *testing.T) {
	cases := []struct {
		scale float64
		want  int
	}{
		{0, uiBasePointSize},  // 非法缩放回退到标准大小
		{-1, uiBasePointSize}, // 负数同样回退
		{0.8, 7},              // 9 × 0.8 = 7.2
		{0.9, 8},              // 9 × 0.9 = 8.1
		{1.0, 9},              // 标准大小
		{1.1, 10},             // 9 × 1.1 = 9.9
		{1.25, 11},            // 9 × 1.25 = 11.25
		{1.5, 14},             // 9 × 1.5 = 13.5
		{0.5, 6},              // 过小则收敛到下限
	}

	for _, c := range cases {
		if got := scaledPointSize(c.scale); got != c.want {
			t.Errorf("scaledPointSize(%g) = %d, 期望 %d", c.scale, got, c.want)
		}
	}
}

// TestFontFamilyOfEntry 验证注册表值名能还原成字体族名。
func TestFontFamilyOfEntry(t *testing.T) {
	cases := []struct {
		entry string
		want  string
	}{
		{"Microsoft YaHei UI (TrueType)", "Microsoft YaHei UI"},
		{"Segoe UI (TrueType)", "Segoe UI"},
		{"MS Shell Dlg 2 (TrueType)", "MS Shell Dlg 2"},
		{"Arial Bold (TrueType)", "Arial Bold"},
		{"SimSun", "SimSun"},
		{"  楷体 (TrueType)  ", "楷体"},
	}

	for _, c := range cases {
		if got := fontFamilyOfEntry(c.entry); got != c.want {
			t.Errorf("fontFamilyOfEntry(%q) = %q, 期望 %q", c.entry, got, c.want)
		}
	}
}
