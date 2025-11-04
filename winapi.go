package main

import (
	"fmt"
	"syscall"
	"unsafe"

	"github.com/lxn/win"

	"golang.org/x/sys/windows"
)

const (
	maxPath = 260 // Maximum path length for Windows file paths
)

var (
	user32  = windows.NewLazySystemDLL("user32.dll")
	shell32 = windows.NewLazySystemDLL("shell32.dll")

	procGetWindowTextW       = user32.NewProc("GetWindowTextW")
	procGetWindowTextLengthW = user32.NewProc("GetWindowTextLengthW")
	procEnumDisplayMonitors  = user32.NewProc("EnumDisplayMonitors")
	procGetKnownFolderPath   = shell32.NewProc("SHGetKnownFolderPath")
)

func enumWindows(callback func(hwnd uintptr, lparam uintptr) uintptr, extra unsafe.Pointer) {
	windows.EnumWindows(windows.NewCallback(callback), extra)
}

func isVisible(hwnd uintptr) bool {
	return win.IsWindowVisible(win.HWND(hwnd))
}

func getWindowTitle(hwnd uintptr) string {
	textLen, _, _ := procGetWindowTextLengthW.Call(hwnd)
	if textLen == 0 {
		return ""
	}

	textBuf := make([]uint16, textLen+1)
	procGetWindowTextW.Call(hwnd, uintptr(unsafe.Pointer(&textBuf[0])), uintptr(len(textBuf)))
	return windows.UTF16ToString(textBuf)
}

func getProcessName(pid uint32) (string, error) {
	handle, err := windows.OpenProcess(windows.PROCESS_QUERY_INFORMATION|windows.PROCESS_VM_READ, false, pid)
	if err != nil {
		return "", err
	}
	defer windows.CloseHandle(handle) // Ensure handle is closed after use
	processNameBuf := make([]uint16, maxPath)
	err = windows.GetModuleBaseName(handle, 0, &processNameBuf[0], maxPath)
	if err != nil {
		return "", err
	}
	processName := windows.UTF16ToString(processNameBuf)
	return processName, nil
}

func getProcessExecutable(pid uint32) (string, error) {
	handle, err := windows.OpenProcess(windows.PROCESS_QUERY_INFORMATION|windows.PROCESS_VM_READ, false, pid)
	if handle == 0 {
		return "", err
	}
	defer windows.CloseHandle(handle) // Ensure handle is closed after use
	exeBuf := make([]uint16, maxPath)
	windows.GetModuleFileNameEx(handle, 0, &exeBuf[0], maxPath)
	exePath := windows.UTF16ToString(exeBuf)
	return exePath, nil
}

func moveWindow(hwnd win.HWND, x, y, width, height int32) {
	win.MoveWindow(hwnd, x, y, width, height, true)
}

func setWindowPos(hwnd win.HWND, x, y, width, height int32) {
	win.SetWindowPos(hwnd, 0, x, y, width, height, win.SWP_NOZORDER)
}

func getWindowRect(hwnd win.HWND) win.RECT {
	rect := win.RECT{}
	win.GetWindowRect(hwnd, &rect)
	return rect
}

func getWindowStyle(hwnd win.HWND) int32 {
	return win.GetWindowLong(hwnd, win.GWL_STYLE)
}

func setWindowStyle(hwnd win.HWND, style int32) {
	win.SetWindowLong(hwnd, win.GWL_STYLE, style)
	win.SetWindowPos(hwnd, 0, 0, 0, 0, 0, win.SWP_NOMOVE|win.SWP_NOSIZE|win.SWP_NOZORDER|win.SWP_FRAMECHANGED)
}

type Monitor struct {
	number    int
	isPrimary bool
	width     int32
	height    int32
	left      int32
	top       int32
}

func (m Monitor) String() string {
	str := fmt.Sprintf("Display %d", m.number)
	if m.isPrimary {
		str += " (Primary)"
	}
	str += fmt.Sprintf(" | %dx%d", m.width, m.height)
	return str
}

var (
	procEnumDisplaySettingsW = user32.NewProc("EnumDisplaySettingsW")
)

type DEVMODE struct {
	DmDeviceName         [32]uint16
	DmSpecVersion        uint16
	DmDriverVersion      uint16
	DmSize               uint16
	DmDriverExtra        uint16
	DmFields             uint32
	DmPosition           struct{ X, Y int32 }
	DmDisplayOrientation uint32
	DmDisplayFixedOutput uint32
	DmColor              int16
	DmDuplex             int16
	DmYResolution        int16
	DmTTOption           int16
	DmCollate            int16
	DmFormName           [32]uint16
	DmLogPixels          uint16
	DmBitsPerPel         uint32
	DmPelsWidth          uint32
	DmPelsHeight         uint32
}

func getMonitors() []Monitor {
	var monitors []Monitor
	index := 0

	cb := syscall.NewCallback(func(hMonitor win.HMONITOR, hdcMonitor win.HDC, lprcMonitor *win.RECT, dwData uintptr) uintptr {
		var infoEx struct {
			win.MONITORINFO
			SzDevice [win.CCHDEVICENAME]uint16
		}
		infoEx.CbSize = uint32(unsafe.Sizeof(infoEx))

		if win.GetMonitorInfo(hMonitor, (*win.MONITORINFO)(unsafe.Pointer(&infoEx))) {
			var devMode DEVMODE
			devMode.DmSize = uint16(unsafe.Sizeof(devMode))

			ret, _, _ := procEnumDisplaySettingsW.Call(
				uintptr(unsafe.Pointer(&infoEx.SzDevice[0])),
				uintptr(0xFFFFFFFF),
				uintptr(unsafe.Pointer(&devMode)),
			)

			width := int32(devMode.DmPelsWidth)
			height := int32(devMode.DmPelsHeight)

			if ret == 0 || width == 0 || height == 0 {
				width = infoEx.RcMonitor.Right - infoEx.RcMonitor.Left
				height = infoEx.RcMonitor.Bottom - infoEx.RcMonitor.Top
			}

			index++
			monitors = append(monitors, Monitor{
				number:    index,
				isPrimary: infoEx.DwFlags&win.MONITORINFOF_PRIMARY != 0,
				width:     width,
				height:    height,
				left:      infoEx.RcMonitor.Left,
				top:       infoEx.RcMonitor.Top,
			})
		}
		return 1
	})
	procEnumDisplayMonitors.Call(0, 0, cb, 0)
	return monitors
}

func getDocumentsFolder() string {
	var buf uintptr
	hr, _, _ := procGetKnownFolderPath.Call(uintptr(unsafe.Pointer(windows.FOLDERID_Documents)), 0, 0, uintptr(unsafe.Pointer(&buf)))
	if hr != 0 {
		return ""
	}
	defer windows.CoTaskMemFree(unsafe.Pointer(buf))                //nolint:govet
	return windows.UTF16PtrToString((*uint16)(unsafe.Pointer(buf))) //nolint:govet
}

func createMutex(name string) (windows.Handle, error) {
	if namePtr, err := windows.UTF16PtrFromString(name); err == nil {
		return windows.CreateMutex(nil, false, namePtr)
	}
	return windows.Handle(0), fmt.Errorf("failed to create mutex")
}

func openMutex(name string) (windows.Handle, error) {
	if namePtr, err := windows.UTF16PtrFromString(name); err == nil {
		return windows.OpenMutex(windows.SYNCHRONIZE, false, namePtr)
	}
	return windows.Handle(0), fmt.Errorf("failed to open mutex")
}

func closeMutex(mutex windows.Handle) error {
	return windows.CloseHandle(mutex)
}

// func waitForSingleObject(mutex *windows.Mutex) error {
// 	return windows.WaitForSingleObject(windows.Handle(mutex), windows.INFINITE)
// }
