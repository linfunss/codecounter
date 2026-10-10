package ui

import (
	"fmt"

	"github.com/lxn/walk"

	"github.com/linfunss/codecounter/internal/counter"
	"github.com/linfunss/codecounter/internal/format"
)

// 条形图的布局常量，单位是与 DPI 无关的逻辑像素。
const (
	chartPadding    = 12
	chartRowHeight  = 30
	chartBadgeSize  = 20
	chartNameWidth  = 150
	chartValueWidth = 96
	chartGap        = 10
	chartRadius     = 4
)

// barChart 负责在自定义控件上绘制语言代码行数条形图。
type barChart struct {
	widget *walk.CustomWidget
	rows   []counter.LanguageStat

	// showIcons 控制是否绘制语言图标。
	showIcons bool
	// maxBars 是图表最多显示的语言数量，0 表示不限。
	maxBars int
}

// newBarChart 创建一个尚未绑定控件的条形图。
func newBarChart() *barChart {
	return &barChart{showIcons: true}
}

// SetRows 更新数据并触发重绘。
func (c *barChart) SetRows(rows []counter.LanguageStat) {
	c.rows = rows
	c.invalidate()
}

// SetOptions 更新显示选项并触发重绘。
func (c *barChart) SetOptions(showIcons bool, maxBars int) {
	c.showIcons = showIcons
	c.maxBars = maxBars
	c.invalidate()
}

// invalidate 在控件已创建时请求重绘。
func (c *barChart) invalidate() {
	if c.widget != nil {
		c.widget.Invalidate()
	}
}

// paint 绘制整张条形图。
func (c *barChart) paint(canvas *walk.Canvas, _ walk.Rectangle) error {
	// 注意：canvas.Bounds() 返回的是屏幕尺寸而非控件尺寸，
	// 绘制范围必须取控件自身的边界，否则布局会被算成整屏宽度。
	bounds := canvas.Bounds()
	if c.widget != nil {
		wb := c.widget.Bounds()
		bounds = walk.Rectangle{Width: wb.Width, Height: wb.Height}
	}

	background, err := walk.NewSolidColorBrush(walk.RGB(0xFF, 0xFF, 0xFF))
	if err != nil {
		return err
	}
	defer background.Dispose()

	if err := canvas.FillRectangle(background, bounds); err != nil {
		return err
	}

	if len(c.rows) == 0 {
		return canvas.DrawText("尚未统计，请选择要统计的目录或文件", c.font(),
			walk.RGB(0x8A, 0x8A, 0x8A), bounds,
			walk.TextCenter|walk.TextVCenter|walk.TextSingleLine|walk.TextNoPrefix)
	}

	rows, hidden := c.visibleRows(bounds)
	if len(rows) == 0 {
		return nil
	}

	maxCode := int64(1)
	for _, r := range rows {
		if r.Code > maxCode {
			maxCode = r.Code
		}
	}

	layout := newChartLayout(bounds, c.showIcons)
	for i, r := range rows {
		if err := c.paintRow(canvas, layout, i, r, maxCode); err != nil {
			return err
		}
	}

	if hidden > 0 {
		rect := walk.Rectangle{
			X:      chartPadding,
			Y:      chartPadding + len(rows)*chartRowHeight,
			Width:  bounds.Width - chartPadding*2,
			Height: chartRowHeight,
		}
		text := fmt.Sprintf("…… 其余 %d 种语言未在图中显示，完整数据见上方排行榜", hidden)
		if err := canvas.DrawText(text, c.font(), walk.RGB(0x8A, 0x8A, 0x8A), rect,
			walk.TextVCenter|walk.TextSingleLine|walk.TextNoPrefix); err != nil {
			return err
		}
	}

	return nil
}

// paintRow 绘制单行：图标、语言名、条形与数值。
func (c *barChart) paintRow(canvas *walk.Canvas, layout chartLayout, index int, stat counter.LanguageStat, maxCode int64) error {
	font := c.font()
	mark := languageBadge(stat.Language)

	y := chartPadding + index*chartRowHeight

	if layout.badgeSize > 0 {
		centerY := y + (chartRowHeight-layout.badgeSize)/2
		badgeRect := walk.Rectangle{
			X:      chartPadding,
			Y:      centerY,
			Width:  layout.badgeSize,
			Height: layout.badgeSize,
		}

		badgeBrush, err := walk.NewSolidColorBrush(mark.color)
		if err != nil {
			return err
		}
		if err := canvas.FillRoundedRectangle(badgeBrush, badgeRect,
			walk.Size{Width: chartRadius + 2, Height: chartRadius + 2}); err != nil {
			badgeBrush.Dispose()
			return err
		}
		badgeBrush.Dispose()

		if err := canvas.DrawText(mark.text, font, contrastColor(mark.color), badgeRect,
			walk.TextCenter|walk.TextVCenter|walk.TextSingleLine|walk.TextNoPrefix); err != nil {
			return err
		}
	}

	nameRect := walk.Rectangle{
		X:      layout.nameX,
		Y:      y,
		Width:  layout.nameWidth,
		Height: chartRowHeight,
	}
	if err := canvas.DrawText(stat.Language, font, walk.RGB(0x1F, 0x1F, 0x1F), nameRect,
		walk.TextVCenter|walk.TextSingleLine|walk.TextEndEllipsis|walk.TextNoPrefix); err != nil {
		return err
	}

	trackRect := walk.Rectangle{
		X:      layout.barX,
		Y:      y + 8,
		Width:  layout.barWidth,
		Height: chartRowHeight - 16,
	}
	track, err := walk.NewSolidColorBrush(walk.RGB(0xEC, 0xEC, 0xEC))
	if err != nil {
		return err
	}
	defer track.Dispose()

	if err := canvas.FillRoundedRectangle(track, trackRect,
		walk.Size{Width: chartRadius, Height: chartRadius}); err != nil {
		return err
	}

	if filled := barLength(stat.Code, maxCode, layout.barWidth); filled > 0 {
		valueRect := walk.Rectangle{X: trackRect.X, Y: trackRect.Y, Width: filled, Height: trackRect.Height}
		value, err := walk.NewSolidColorBrush(mark.color)
		if err != nil {
			return err
		}
		defer value.Dispose()

		if err := canvas.FillRoundedRectangle(value, valueRect,
			walk.Size{Width: chartRadius, Height: chartRadius}); err != nil {
			return err
		}
	}

	numberRect := walk.Rectangle{
		X:      layout.barX + layout.barWidth + chartGap,
		Y:      y,
		Width:  layout.valueWidth,
		Height: chartRowHeight,
	}
	return canvas.DrawText(format.Int(stat.Code), font, walk.RGB(0x1F, 0x1F, 0x1F), numberRect,
		walk.TextRight|walk.TextVCenter|walk.TextSingleLine|walk.TextNoPrefix)
}

// visibleRows 返回当前显示选项与尺寸下能显示的行，以及未显示的行数。
func (c *barChart) visibleRows(bounds walk.Rectangle) ([]counter.LanguageStat, int) {
	rows := c.rows
	if c.maxBars > 0 && len(rows) > c.maxBars {
		rows = rows[:c.maxBars]
	}

	maxRows := (bounds.Height - chartPadding*2) / chartRowHeight
	if maxRows < 1 {
		maxRows = 1
	}

	if len(rows) <= maxRows {
		return rows, len(c.rows) - len(rows)
	}

	if maxRows > 1 {
		// 预留一行用于提示还有多少语言未显示
		maxRows--
	}
	return rows[:maxRows], len(c.rows) - maxRows
}

// font 返回绘图使用的字体，直接复用控件的系统默认字体。
func (c *barChart) font() *walk.Font {
	if c.widget != nil {
		return c.widget.Font()
	}
	return nil
}

// chartLayout 记录条形图中各列的横向位置。
type chartLayout struct {
	badgeSize  int
	nameX      int
	nameWidth  int
	barX       int
	barWidth   int
	valueWidth int
}

// newChartLayout 依据可用宽度计算各列位置，窗口过窄时自动压缩语言名宽度。
func newChartLayout(bounds walk.Rectangle, showIcons bool) chartLayout {
	badgeSize := 0
	iconWidth := 0
	if showIcons {
		badgeSize = chartBadgeSize
		iconWidth = chartBadgeSize + chartGap
	}

	nameX := chartPadding + iconWidth

	nameWidth := chartNameWidth
	if limit := (bounds.Width - iconWidth) / 4; nameWidth > limit {
		nameWidth = limit
	}
	if nameWidth < 60 {
		nameWidth = 60
	}

	barX := func(width int) int { return nameX + width + chartGap }

	barWidth := bounds.Width - barX(nameWidth) - chartValueWidth - chartPadding
	if barWidth < 60 {
		nameWidth -= 60 - barWidth
		if nameWidth < 60 {
			nameWidth = 60
		}
		barWidth = bounds.Width - barX(nameWidth) - chartValueWidth - chartPadding
	}
	if barWidth < 20 {
		barWidth = 20
	}

	return chartLayout{
		badgeSize:  badgeSize,
		nameX:      nameX,
		nameWidth:  nameWidth,
		barX:       barX(nameWidth),
		barWidth:   barWidth,
		valueWidth: chartValueWidth,
	}
}

// barLength 按比例计算条形长度，保证有数据的语言至少显示一小段。
func barLength(code, maxCode int64, width int) int {
	if code <= 0 || width <= 0 || maxCode <= 0 {
		return 0
	}

	filled := int(float64(width) * float64(code) / float64(maxCode))
	if filled < 4 {
		filled = 4
	}
	if filled > width {
		filled = width
	}
	return filled
}
