//go:build windows

package ui

import (
	"syscall"
	"unsafe"

	"github.com/lxn/walk"
	"golang.org/x/sys/windows"
)

// Windows COM 相关常量。
const (
	clsctxInprocServer = 1
	coinitApartment    = 0x2
	sFalse             = 0x1
	rpcEChangedMode    = 0x80010106
	fosPickFolders     = 0x00000020
	fosForceFileSystem = 0x00000040
	fosPathMustExist   = 0x00000800
	sigdnFileSysPath   = 0x80058000
)

// oleGUID 对应 Windows 的 GUID 结构。
type oleGUID struct {
	Data1 uint32
	Data2 uint16
	Data3 uint16
	Data4 [8]byte
}

var (
	clsidFileOpenDialog = oleGUID{0xDC1C5A9C, 0xE88A, 0x4DDE, [8]byte{0xA5, 0xA1, 0x60, 0xF8, 0x2A, 0x20, 0xAE, 0xF7}}
	iidFileOpenDialog   = oleGUID{0xD57C7288, 0xD4AD, 0x4768, [8]byte{0xBE, 0x02, 0x9D, 0x96, 0x95, 0x32, 0xD9, 0x60}}
	iidShellItem        = oleGUID{0x43826D1E, 0xE718, 0x42EE, [8]byte{0xBC, 0x55, 0xA1, 0xE2, 0x61, 0xC3, 0x7B, 0xFE}}
)

var (
	ole32                           = syscall.NewLazyDLL("ole32.dll")
	shell32                         = syscall.NewLazyDLL("shell32.dll")
	procCoInitializeEx              = ole32.NewProc("CoInitializeEx")
	procCoUninitialize              = ole32.NewProc("CoUninitialize")
	procCoCreateInstance            = ole32.NewProc("CoCreateInstance")
	procCoTaskMemFree               = ole32.NewProc("CoTaskMemFree")
	procSHCreateItemFromParsingName = shell32.NewProc("SHCreateItemFromParsingName")
)

// IFileOpenDialog 与 IShellItem 的 vtable 下标。
// 这些接口在 shobjidl.h 中的顺序是固定的，取值不随系统版本变化。
const (
	// IUnknown
	methodRelease = 2
	// IModalWindow
	methodShow = 3
	// IFileDialog
	methodSetOptions = 9
	methodSetFolder  = 12
	methodSetTitle   = 17
	methodGetResult  = 20
	// IShellItem
	methodGetDisplayName = 5
)

// comCall 调用 COM 接口方法，index 是 vtable 下标（QueryInterface 为 0）。
//
// iface 以 unsafe.Pointer 保存，只在最后一刻转换为 uintptr，
// 避免 unsafe.Pointer 与 uintptr 之间的往返转换。
func comCall(iface unsafe.Pointer, index int, args ...uintptr) uintptr {
	// 接口指针指向的第一个指针就是 vtable
	vtbl := *(*unsafe.Pointer)(iface)
	fn := *(*uintptr)(unsafe.Add(vtbl, uintptr(index)*unsafe.Sizeof(uintptr(0))))

	callArgs := make([]uintptr, 0, len(args)+1)
	callArgs = append(callArgs, uintptr(iface))
	callArgs = append(callArgs, args...)

	result, _, _ := syscall.SyscallN(fn, callArgs...)
	return result
}

// comRelease 释放一个 COM 接口引用。
func comRelease(iface unsafe.Pointer) {
	if iface != nil {
		comCall(iface, methodRelease)
	}
}

// chooseFolder 使用 Windows 现代文件夹选择对话框选取目录。
//
// 相比旧版 SHBrowseForFolder，该对话框支持在整个计算机范围内浏览，
// 可以直接从地址栏或侧边栏跳转到其它磁盘。用户取消时返回 ok=false。
func chooseFolder(owner walk.Form, title, initialDir string) (dir string, ok bool) {
	// S_FALSE 表示当前线程已经初始化过 COM；
	// RPC_E_CHANGED_MODE 表示已用其它套间初始化，此时沿用现有套间即可。
	hr, _, _ := procCoInitializeEx.Call(0, coinitApartment)
	if hr == 0 {
		defer func() { _, _, _ = procCoUninitialize.Call() }()
	} else if hr != sFalse && hr != rpcEChangedMode {
		return "", false
	}

	var dialog unsafe.Pointer
	hr, _, _ = procCoCreateInstance.Call(
		uintptr(unsafe.Pointer(&clsidFileOpenDialog)),
		0,
		clsctxInprocServer,
		uintptr(unsafe.Pointer(&iidFileOpenDialog)),
		uintptr(unsafe.Pointer(&dialog)),
	)
	if hr != 0 || dialog == nil {
		return "", false
	}
	defer comRelease(dialog)

	// 只允许选择已存在的文件夹，并且必须是文件系统路径
	if comCall(dialog, methodSetOptions, uintptr(fosPickFolders|fosForceFileSystem|fosPathMustExist)) != 0 {
		return "", false
	}

	if title != "" {
		if titlePtr, err := syscall.UTF16PtrFromString(title); err == nil {
			comCall(dialog, methodSetTitle, uintptr(unsafe.Pointer(titlePtr)))
		}
	}

	// 指定初始目录，让对话框从当前统计目录打开
	if initialDir != "" {
		if item, created := shellItemFromPath(initialDir); created {
			comCall(dialog, methodSetFolder, uintptr(item))
			comRelease(item)
		}
	}

	var ownerHwnd uintptr
	if owner != nil {
		ownerHwnd = uintptr(owner.Handle())
	}

	if comCall(dialog, methodShow, ownerHwnd) != 0 {
		// 用户取消时返回 HRESULT_FROM_WIN32(ERROR_CANCELLED)
		return "", false
	}

	var result unsafe.Pointer
	if comCall(dialog, methodGetResult, uintptr(unsafe.Pointer(&result))) != 0 || result == nil {
		return "", false
	}
	defer comRelease(result)

	var namePtr *uint16
	if comCall(result, methodGetDisplayName, sigdnFileSysPath, uintptr(unsafe.Pointer(&namePtr))) != 0 || namePtr == nil {
		return "", false
	}
	defer func() { _, _, _ = procCoTaskMemFree.Call(uintptr(unsafe.Pointer(namePtr))) }()

	return windows.UTF16PtrToString(namePtr), true
}

// shellItemFromPath 由文件系统路径创建 IShellItem。
func shellItemFromPath(path string) (unsafe.Pointer, bool) {
	pathPtr, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return nil, false
	}

	var item unsafe.Pointer
	hr, _, _ := procSHCreateItemFromParsingName.Call(
		uintptr(unsafe.Pointer(pathPtr)),
		0,
		uintptr(unsafe.Pointer(&iidShellItem)),
		uintptr(unsafe.Pointer(&item)),
	)
	if hr != 0 || item == nil {
		return nil, false
	}
	return item, true
}
