package ui

import (
	"github.com/lxn/walk"

	"github.com/linfunss/codecounter/internal/counter"
	"github.com/linfunss/codecounter/internal/format"
)

// leaderboardModel 为排行榜表格提供数据。
//
// 数值列返回的是已经带千位分隔符的字符串，配合列右对齐即可获得
// 与原生控件一致的观感。
type leaderboardModel struct {
	walk.TableModelBase
	rows []counter.LanguageStat
}

// RowCount 返回表格行数。
func (m *leaderboardModel) RowCount() int {
	return len(m.rows)
}

// Value 返回指定单元格应显示的内容。
func (m *leaderboardModel) Value(row, col int) interface{} {
	if row < 0 || row >= len(m.rows) {
		return ""
	}

	stat := m.rows[row]
	switch col {
	case 0:
		return row + 1
	case 1:
		return stat.Language
	case 2:
		return format.Int(int64(stat.Files))
	case 3:
		return format.Int(stat.Code)
	case 4:
		return format.Int(stat.Comment)
	case 5:
		return format.Int(stat.Blank)
	case 6:
		return format.Int(stat.Lines)
	}
	return ""
}

// SetRows 用新的统计结果替换表格内容。
func (m *leaderboardModel) SetRows(rows []counter.LanguageStat) {
	m.rows = rows
	m.PublishRowsReset()
}
