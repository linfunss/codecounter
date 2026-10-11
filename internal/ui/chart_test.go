package ui

import (
	"testing"

	"github.com/lxn/walk"

	"github.com/linfunss/codecounter/internal/counter"
)

// TestBarLength 验证条形长度按比例缩放并带最小值保护。
func TestBarLength(t *testing.T) {
	cases := []struct {
		code  int64
		max   int64
		width int
		want  int
	}{
		{0, 100, 200, 0},
		{100, 100, 200, 200},
		{50, 100, 200, 100},
		{1, 1000, 200, 1},
		{200, 100, 200, 200}, // 超过最大值时截断到可用宽度
		{10, 0, 200, 0},
		{10, 100, 0, 0},
	}

	for _, c := range cases {
		if got := barLength(c.code, c.max, c.width); got != c.want {
			t.Errorf("barLength(%d, %d, %d) = %d, 期望 %d", c.code, c.max, c.width, got, c.want)
		}
	}
}

// TestVisibleRows 验证行数超出可视范围时的裁剪与提示计数。
func TestVisibleRows(t *testing.T) {
	chart := newBarChart()
	rows := make([]counter.LanguageStat, 0)
	for i := 0; i < 10; i++ {
		rows = append(rows, counter.LanguageStat{Language: "Go", Code: int64(i + 1)})
	}
	chart.SetRows(rows)

	// 高度足够时全部显示
	bounds := walk.Rectangle{Width: 800, Height: 1000}
	visible, hidden := chart.visibleRows(bounds, newChartMetrics(1))
	if len(visible) != 10 || hidden != 0 {
		t.Fatalf("高度充足时应全部显示，实际显示 %d 行，隐藏 %d 行", len(visible), hidden)
	}

	// 高度受限时裁剪，并预留一行提示
	bounds = walk.Rectangle{Width: 800, Height: 150}
	visible, hidden = chart.visibleRows(bounds, newChartMetrics(1))
	if hidden != 0 || len(visible) != 10 {
		t.Error("滚动内容必须保留全部语言")
	}
	if len(visible)+hidden != 10 {
		t.Errorf("显示 %d 行 + 隐藏 %d 行 != 总数 10", len(visible), hidden)
	}
}

// TestNewChartLayoutFitsBounds 验证常见宽度下布局不会越界。
func TestNewChartLayoutFitsBounds(t *testing.T) {
	for _, width := range []int{700, 880, 1040, 1600} {
		bounds := walk.Rectangle{Width: width, Height: 400}
		layout := newChartLayout(bounds, true, newChartMetrics(1))

		if layout.barWidth < 20 {
			t.Errorf("宽度 %d 时条形宽度过小: %d", width, layout.barWidth)
		}
		if layout.nameWidth < 60 {
			t.Errorf("宽度 %d 时语言名宽度过小: %d", width, layout.nameWidth)
		}
		if end := layout.barX + layout.barWidth + layout.valueWidth; end > width {
			t.Errorf("宽度 %d 时布局越界: 右端 %d", width, end)
		}
	}
}
