package ui

import (
	"fmt"

	"github.com/lxn/walk"

	"github.com/linfunss/codecounter/internal/counter"
	"github.com/linfunss/codecounter/internal/format"
)

// chartBaseMetrics 是缩放系数为 1.0 时的条形图尺寸，单位是与 DPI 无关的逻辑像素。
// 取值偏紧凑，避免出现大片空白。
var chartBaseMetrics = struct {
	padding    int
	rowHeight  int
	badgeSize  int
	nameWidth  int
	valueWidth int
	gap        int
	radius     int
}{
	padding:    8,
	rowHeight:  26,
	badgeSize:  18,
	nameWidth:  128,
	valueWidth: 128, // 需要容纳「1,234 (45.2%)」
	gap:        8,
	radius:     4,
}

// chartMetrics 是按当前缩放系数换算后的尺寸。
type chartMetrics struct {
	padding    int
	rowHeight  int
	badgeSize  int
	nameWidth  int
	valueWidth int
	gap        int
	radius     int
}

// newChartMetrics 依据缩放系数计算布局尺寸。
func newChartMetrics(scale float64) chartMetrics {
	if scale <= 0 {
		scale = 1
	}

	scaleInt := func(base int) int {
		value := int(float64(base)*scale + 0.5)
		if value < 1 {
			value = 1
		}
		return value
	}

	return chartMetrics{
		padding:    scaleInt(chartBaseMetrics.padding),
		rowHeight:  scaleInt(chartBaseMetrics.rowHeight),
		badgeSize:  scaleInt(chartBaseMetrics.badgeSize),
		nameWidth:  scaleInt(chartBaseMetrics.nameWidth),
		valueWidth: scaleInt(chartBaseMetrics.valueWidth),
		gap:        scaleInt(chartBaseMetrics.gap),
		radius:     scaleInt(chartBaseMetrics.radius),
	}
}

// barChart 负责在自定义控件上绘制语言代码行数条形图。
type barChart struct {
	scroll *walk.ScrollView
	widget *walk.CustomWidget
	rows   []counter.LanguageStat

	// showIcons 控制是否绘制语言图标。
	showIcons bool
	// showPercent 控制是否显示代码行数占比。
	showPercent bool
	// maxBars 是图表最多显示的语言数量，0 表示不限。
	maxBars int
	// scale 是界面缩放系数。
	scale float64
}

// newBarChart 创建一个尚未绑定控件的条形图。
func newBarChart() *barChart {
	return &barChart{showIcons: true, showPercent: true, scale: 1}
}

// SetRows 更新数据并触发重绘。
func (c *barChart) SetRows(rows []counter.LanguageStat) {
	c.rows = rows
	c.invalidate()
}

// SetOptions 更新显示选项并触发重绘。
func (c *barChart) SetOptions(showIcons, showPercent bool, maxBars int, scale float64) {
	c.showIcons = showIcons
	c.showPercent = showPercent
	c.maxBars = 0
	c.scale = scale
	c.invalidate()
}

// invalidate 在控件已创建时请求重绘。
func (c *barChart) invalidate() {
	if c.widget != nil {
		m := newChartMetrics(c.scale)
		_ = c.widget.SetMinMaxSize(walk.Size{Height: m.padding*2 + len(c.rows)*m.rowHeight}, walk.Size{})
		// 重绘请求失败没有补救手段，忽略返回值
		_ = c.widget.Invalidate()
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

	m := newChartMetrics(c.scale)
	dpi := canvas.DPI()

	background, err := walk.NewSolidColorBrush(walk.RGB(0xFF, 0xFF, 0xFF))
	if err != nil {
		return err
	}
	defer background.Dispose()

	if err := canvas.FillRectanglePixels(background, walk.RectangleFrom96DPI(bounds, dpi)); err != nil {
		return err
	}

	if len(c.rows) == 0 {
		return c.drawText(canvas, "尚未统计，请选择要统计的目录或文件",
			walk.RGB(0x8A, 0x8A, 0x8A), bounds,
			walk.TextCenter|walk.TextVCenter|walk.TextSingleLine|walk.TextNoPrefix)
	}

	rows, hidden := c.visibleRows(bounds, m)
	if len(rows) == 0 {
		return nil
	}

	maxCode := int64(1)
	totalCode := int64(0)
	for _, r := range c.rows {
		totalCode += r.Code
	}
	for _, r := range rows {
		if r.Code > maxCode {
			maxCode = r.Code
		}
	}

	// ScrollView 的内容宽度可能包含被滚动条裁掉的区域。
	// 使用视口宽度，并始终保留右侧安全空间。
	layoutBounds := bounds
	if c.scroll != nil {
		viewportWidth := c.scroll.ClientBounds().Width - 18
		if viewportWidth > 0 && viewportWidth < layoutBounds.Width {
			layoutBounds.Width = viewportWidth
		}
	}
	layout := newChartLayout(layoutBounds, c.showIcons, m)
	for i, r := range rows {
		if err := c.paintRow(canvas, layout, m, i, r, maxCode, totalCode); err != nil {
			return err
		}
	}

	if hidden > 0 {
		rect := walk.Rectangle{
			X:      m.padding,
			Y:      m.padding + len(rows)*m.rowHeight,
			Width:  bounds.Width - m.padding*2,
			Height: m.rowHeight,
		}
		text := fmt.Sprintf("…… 其余 %d 种语言未在图中显示，完整数据见上方排行榜", hidden)
		if err := c.drawText(canvas, text, walk.RGB(0x8A, 0x8A, 0x8A), rect,
			walk.TextVCenter|walk.TextSingleLine|walk.TextNoPrefix); err != nil {
			return err
		}
	}

	return nil
}

// drawText 按逻辑坐标绘制文本，内部换算为物理像素。
func (c *barChart) drawText(canvas *walk.Canvas, text string, color walk.Color, bounds walk.Rectangle, textFormat walk.DrawTextFormat) error {
	return canvas.DrawTextPixels(text, c.font(), color, walk.RectangleFrom96DPI(bounds, canvas.DPI()), textFormat)
}

// fillRounded 按逻辑坐标填充圆角矩形，内部换算为物理像素。
func (c *barChart) fillRounded(canvas *walk.Canvas, brush walk.Brush, bounds walk.Rectangle, radius int) error {
	dpi := canvas.DPI()
	return canvas.FillRoundedRectanglePixels(brush,
		walk.RectangleFrom96DPI(bounds, dpi),
		walk.SizeFrom96DPI(walk.Size{Width: radius, Height: radius}, dpi))
}

// paintBadge 绘制自绘的语言徽章，用于没有内置图标的语言。
func (c *barChart) paintBadge(canvas *walk.Canvas, m chartMetrics, bounds walk.Rectangle, mark badge) error {
	brush, err := walk.NewSolidColorBrush(mark.color)
	if err != nil {
		return err
	}
	defer brush.Dispose()

	if err := c.fillRounded(canvas, brush, bounds, m.radius+2); err != nil {
		return err
	}

	return c.drawText(canvas, mark.text, contrastColor(mark.color), bounds,
		walk.TextCenter|walk.TextVCenter|walk.TextSingleLine|walk.TextNoPrefix)
}

// paintRow 绘制单行：图标、语言名、条形与数值。
func (c *barChart) paintRow(canvas *walk.Canvas, layout chartLayout, m chartMetrics, index int, stat counter.LanguageStat, maxCode, totalCode int64) error {
	mark := languageBadge(stat.Language)

	y := m.padding + index*m.rowHeight

	if layout.badgeSize > 0 {
		centerY := y + (m.rowHeight-layout.badgeSize)/2
		badgeRect := walk.Rectangle{
			X:      m.padding,
			Y:      centerY,
			Width:  layout.badgeSize,
			Height: layout.badgeSize,
		}

		if logo := languageLogo(stat.Language); logo != nil {
			// 按目标像素尺寸绘制，避免 GDI 缩放导致图标模糊
			pixelRect := walk.RectangleFrom96DPI(badgeRect, canvas.DPI())
			if err := canvas.DrawImageStretchedPixels(logo, pixelRect); err != nil {
				return err
			}
		} else if err := c.paintBadge(canvas, m, badgeRect, mark); err != nil {
			return err
		}
	}

	nameRect := walk.Rectangle{
		X:      layout.nameX,
		Y:      y,
		Width:  layout.nameWidth,
		Height: m.rowHeight,
	}
	if err := c.drawText(canvas, stat.Language, walk.RGB(0x1F, 0x1F, 0x1F), nameRect,
		walk.TextVCenter|walk.TextSingleLine|walk.TextEndEllipsis|walk.TextNoPrefix); err != nil {
		return err
	}

	barHeight := m.rowHeight - m.gap*2
	if barHeight < 4 {
		barHeight = 4
	}
	trackRect := walk.Rectangle{
		X:      layout.barX,
		Y:      y + (m.rowHeight-barHeight)/2,
		Width:  layout.barWidth,
		Height: barHeight,
	}
	track, err := walk.NewSolidColorBrush(walk.RGB(0xEC, 0xEC, 0xEC))
	if err != nil {
		return err
	}
	defer track.Dispose()

	if err := c.fillRounded(canvas, track, trackRect, m.radius); err != nil {
		return err
	}

	if filled := barLength(stat.Code, totalCode, layout.barWidth); filled > 0 {
		valueRect := walk.Rectangle{X: trackRect.X, Y: trackRect.Y, Width: filled, Height: trackRect.Height}
		value, err := walk.NewSolidColorBrush(mark.color)
		if err != nil {
			return err
		}
		defer value.Dispose()

		if err := c.fillRounded(canvas, value, valueRect, m.radius); err != nil {
			return err
		}
	}

	numberRect := walk.Rectangle{
		X:      layout.barX + layout.barWidth + m.gap,
		Y:      y,
		Width:  layout.valueWidth,
		Height: m.rowHeight,
	}
	return c.drawText(canvas, chartValueText(stat.Code, totalCode, c.showPercent),
		walk.RGB(0x1F, 0x1F, 0x1F), numberRect,
		walk.TextRight|walk.TextVCenter|walk.TextSingleLine|walk.TextNoPrefix)
}

// chartValueText 组合条形图右侧的数值与占比。
func chartValueText(code, total int64, showPercent bool) string {
	if !showPercent || total <= 0 {
		return format.Int(code)
	}
	return fmt.Sprintf("%s (%.1f%%)", format.Int(code), float64(code)/float64(total)*100)
}

// visibleRows 返回当前显示选项与尺寸下能显示的行，以及未显示的行数。
func (c *barChart) visibleRows(bounds walk.Rectangle, m chartMetrics) ([]counter.LanguageStat, int) {
	return c.rows, 0
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
func newChartLayout(bounds walk.Rectangle, showIcons bool, m chartMetrics) chartLayout {
	badgeSize := 0
	iconWidth := 0
	if showIcons {
		badgeSize = m.badgeSize
		iconWidth = m.badgeSize + m.gap
	}

	nameX := m.padding + iconWidth

	// 语言名最多占可用宽度的四分之一，避免挤掉条形图
	nameWidth := m.nameWidth
	if limit := (bounds.Width - iconWidth) / 4; nameWidth > limit {
		nameWidth = limit
	}
	if nameWidth < m.gap*5 {
		nameWidth = m.gap * 5
	}

	barX := func(width int) int { return nameX + width + m.gap }

	barWidth := bounds.Width - barX(nameWidth) - m.valueWidth - m.gap - m.padding*2
	if barWidth < m.gap*5 {
		nameWidth -= m.gap*5 - barWidth
		if nameWidth < m.gap*5 {
			nameWidth = m.gap * 5
		}
		barWidth = bounds.Width - barX(nameWidth) - m.valueWidth - m.gap - m.padding*2
	}
	if barWidth < m.gap*2 {
		barWidth = m.gap * 2
	}

	return chartLayout{
		badgeSize:  badgeSize,
		nameX:      nameX,
		nameWidth:  nameWidth,
		barX:       barX(nameWidth),
		barWidth:   barWidth,
		valueWidth: m.valueWidth,
	}
}

// barLength 按比例计算条形长度，保证有数据的语言至少显示一小段。
func barLength(code, maxCode int64, width int) int {
	if code <= 0 || width <= 0 || maxCode <= 0 {
		return 0
	}

	filled := int(float64(width) * float64(code) / float64(maxCode))
	if filled < 1 {
		filled = 1
	}
	if filled > width {
		filled = width
	}
	return filled
}
