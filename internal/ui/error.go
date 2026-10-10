package ui

import (
	"github.com/lxn/walk"
)

// ShowError 在图形界面下弹出错误提示。
// 无窗口可用时（例如启动早期失败）退化为系统消息框。
func ShowError(title string, err error) {
	if err == nil {
		return
	}
	walk.MsgBox(nil, title, err.Error(), walk.MsgBoxIconError)
}
