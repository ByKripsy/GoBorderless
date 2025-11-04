package main

import (
	_ "embed"
	"fmt"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"unsafe"

	"github.com/adamk33n3r/GoBorderless/rx"
	"github.com/adamk33n3r/GoBorderless/ui"
	"github.com/lxn/win"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

var (
	applicationSelect *ui.Select[Window]
	displaySelect     *ui.Select[Monitor]
	matchType         *widget.RadioGroup
	xOffsetText       *widget.Entry
	yOffsetText       *widget.Entry
	widthText         *widget.Entry
	heightText        *widget.Entry
	confirmButton     *widget.Button

	halfLeftBtn  *widget.Button
	halfRightBtn *widget.Button
	fullBtn      *widget.Button
)

func isValid(isNew bool) bool {
	return (!isNew || (applicationSelect != nil && applicationSelect.Selected != nil)) &&
		displaySelect != nil && displaySelect.Selected != nil &&
		matchType != nil && matchType.Selected != "" &&
		xOffsetText != nil && xOffsetText.Validate() == nil &&
		yOffsetText != nil && yOffsetText.Validate() == nil &&
		widthText != nil && widthText.Validate() == nil &&
		heightText != nil && heightText.Validate() == nil
}

func setConfirmButtonState(isNew bool) {
	if isValid(isNew) {
		confirmButton.Enable()
	} else {
		confirmButton.Disable()
	}
}

func entryTextToInt(s string) int32 {
	intVal, _ := strconv.Atoi(s)
	return int32(intVal)
}

func setViaReflect(obj any, fieldName string, val reflect.Value) {
	rs := reflect.ValueOf(obj).Elem()
	rf := rs.FieldByName(fieldName)
	rf = reflect.NewAt(rf.Type(), unsafe.Pointer(rf.UnsafeAddr())).Elem()
	rf.Set(val)
}

func setOnFocusChanged(entry *widget.Entry, onFocusChanged func(focused bool)) {
	setViaReflect(entry, "onFocusChanged", reflect.ValueOf(onFocusChanged))
}

func getWindowsForSelect(allWindows []Window) []Window {
	copyOfWindows := make([]Window, 0, len(allWindows))
	// Filter out windows that don't have normal borders cause they're probably not "real" windows
	// This will also filter out windows that we've already removed borders from
	// Perhaps we should also check the list of existing configs?
	for _, window := range allWindows {
		if isValidWindowForSelection(window) {
			copyOfWindows = append(copyOfWindows, window)
		}
	}
	slices.SortFunc(copyOfWindows, func(a, b Window) int {
		return strings.Compare(strings.ToLower(a.String()), strings.ToLower(b.String()))
	})
	return copyOfWindows
}

func isValidWindowForSelection(window Window) bool {
	style := getWindowStyle(window.hwnd)
	return style&win.WS_CAPTION > 0 &&
		((style&win.WS_BORDER) > 0 || (style&win.WS_THICKFRAME) > 0)
}

func createApplicationSelect(windowsForSelect []Window, appSetting *AppSetting, isNew bool) *ui.Select[Window] {
	applicationSelect = ui.NewSelect(windowsForSelect, func(selected Window) {
		if slices.Index(windowsForSelect, selected) == -1 {
			fmt.Println("Selected application no longer exists in the updated window list, resetting selection.")
			applicationSelect.ClearSelected()
			return
		}
		fmt.Println("Selected Application:", selected)
		appSetting.WindowName = selected.title
		appSetting.ExePath = selected.exePath

		setConfirmButtonState(isNew)
	})
	applicationSelect.PlaceHolder = "Select Application"

	return applicationSelect
}

func getDefaultMonitorIndex(settings *Settings, appSetting AppSetting, isNew bool) int {
	if !isNew {
		return appSetting.Monitor - 1
	}

	monitorIdx := settings.Defaults.Monitor - 1
	if monitorIdx < 0 {
		monitorIdx = slices.IndexFunc(monitors, func(m Monitor) bool {
			return m.isPrimary
		})
	}

	return monitorIdx
}

func createDisplaySelect(settings *Settings, appSetting *AppSetting, isNew bool) *ui.Select[Monitor] {
	monitorIdx := getDefaultMonitorIndex(settings, *appSetting, isNew)

	displaySelect = ui.NewSelect(monitors, func(selected Monitor) {
		appSetting.Monitor = selected.number
		setConfirmButtonState(isNew)
	})
	displaySelect.PlaceHolder = "Select Display"
	displaySelect.SetSelectedIndex(monitorIdx)

	return displaySelect
}

func createMatchTypeRadio(settings *Settings, appSetting *AppSetting, isNew bool) *widget.RadioGroup {
	matchType = widget.NewRadioGroup(matchTypes, func(selected string) {
		appSetting.MatchType = GetMatchTypeFromString(selected)
		setConfirmButtonState(isNew)
	})
	if isNew {
		matchType.SetSelected(settings.Defaults.MatchType.String())
	} else {
		matchType.SetSelected(appSetting.MatchType.String())
	}
	matchType.Horizontal = true
	matchType.Required = true

	return matchType
}

func createSizeButton(monitor Monitor, xDivider int32, xOffset int32, label string, icon fyne.Resource) *widget.Button {
	monitorWidth := monitor.width / xDivider
	offsetWidth := int32(0)
	if xOffset == 1 {
		offsetWidth = monitorWidth
	}

	return widget.NewButtonWithIcon(label, icon, func() {
		widthText.SetText(strconv.Itoa(int(monitorWidth)))
		heightText.SetText(strconv.Itoa(int(monitor.height)))
		xOffsetText.SetText(strconv.Itoa(int(offsetWidth)))
		yOffsetText.SetText("0")
	})
}

func createOffsetEntry(label string, defaultValue int32, settings *Settings, appSetting *AppSetting, isNew bool, updateField func(int32)) (*widget.Label, *widget.Entry) {
	labelWidget := widget.NewLabel(label)

	entry := widget.NewEntry()
	entry.Validator = offsetIntValidator
	entry.OnChanged = func(s string) {
		if s == "" {
			updateField(0)
		} else {
			updateField(entryTextToInt(s))
		}
		setConfirmButtonState(isNew)
	}
	setOnFocusChanged(entry, func(focused bool) {
		if focused {
			entry.DoubleTapped(&fyne.PointEvent{})
		}
	})

	entry.SetPlaceHolder("0")
	if isNew {
		entry.SetText(strconv.Itoa(int(defaultValue)))
	} else {
		entry.SetText(strconv.Itoa(int(defaultValue)))
	}

	return labelWidget, entry
}

func createSizeEntry(label string, placeholder string, defaultValue int32, settings *Settings, appSetting *AppSetting, isNew bool, updateField func(int32)) (*widget.Label, *widget.Entry) {
	labelWidget := widget.NewLabel(label)

	entry := widget.NewEntry()
	entry.Validator = intValidator
	entry.OnChanged = func(s string) {
		updateField(entryTextToInt(s))
		setConfirmButtonState(isNew)
	}

	setOnFocusChanged(entry, func(focused bool) {
		if focused {
			entry.DoubleTapped(&fyne.PointEvent{})
		}
	})

	entry.SetPlaceHolder(placeholder)
	if isNew {
		entry.SetText(strconv.Itoa(int(defaultValue)))
	} else {
		entry.SetText(strconv.Itoa(int(defaultValue)))
	}

	return labelWidget, entry
}

func createTextGrid(settings *Settings, appSetting *AppSetting, isNew bool) *fyne.Container {
	xOffsetLabel, xOffsetEntry := createOffsetEntry("X Offset:", settings.Defaults.OffsetX, settings, appSetting, isNew, func(val int32) {
		appSetting.OffsetX = val
	})
	xOffsetText = xOffsetEntry

	yOffsetLabel, yOffsetEntry := createOffsetEntry("Y Offset:", settings.Defaults.OffsetY, settings, appSetting, isNew, func(val int32) {
		appSetting.OffsetY = val
	})
	yOffsetText = yOffsetEntry

	widthLabel, widthEntry := createSizeEntry("Width:", "1920", settings.Defaults.Width, settings, appSetting, isNew, func(val int32) {
		appSetting.Width = val
	})
	widthText = widthEntry

	heightLabel, heightEntry := createSizeEntry("Height:", "1080", settings.Defaults.Height, settings, appSetting, isNew, func(val int32) {
		appSetting.Height = val
	})
	heightText = heightEntry

	return container.NewGridWithRows(2,
		container.NewGridWithColumns(2,
			container.NewVBox(xOffsetLabel, xOffsetText),
			container.NewVBox(yOffsetLabel, yOffsetText),
		),
		container.NewGridWithColumns(2,
			container.NewVBox(widthLabel, widthText),
			container.NewVBox(heightLabel, heightText),
		),
	)
}

func subscribeToWindowUpdates(windowsForSelect []Window, isNew bool) rx.Subscription {
	if !isNew {
		return rx.Subscription{}
	}

	fmt.Println("subscribing to windows observable")
	// TODO: make it work like subject where it outputs last received data on subscription

	return windowObs.Subscribe(func(windows []Window) {
		if len(windows) == 0 {
			// This is probably a fluke, so let's skip it
			return
		}

		fyne.Do(func() {
			windowsForSelect = getWindowsForSelect(windows)
			applicationSelect.SetOptions(windowsForSelect)

			if applicationSelect.Selected != nil && slices.Index(windowsForSelect, *applicationSelect.Selected) == -1 {
				fmt.Println("Selected application no longer exists in the updated window list, resetting selection.")
				applicationSelect.ClearSelected()
			}
		})
	})
}

func createPresetsContent(settings *Settings, appSetting *AppSetting, isNew bool, cancelButton, confirmBtn *widget.Button) *fyne.Container {
	presetsRow := container.NewCenter(
		container.NewHBox(
			halfLeftBtn,
			widget.NewLabel("   "),
			halfRightBtn,
			widget.NewLabel("   "),
			fullBtn,
		),
	)

	content := container.NewVBox(
		displaySelect,
		widget.NewLabel("Match Type"),
		matchType,
		widget.NewLabel("Presets:"),
		presetsRow,
		createTextGrid(settings, appSetting, isNew),
		widget.NewLabel(""), // spacer
		container.NewHBox(cancelButton, layout.NewSpacer(), confirmBtn),
	)

	if isNew {
		content.Objects = append([]fyne.CanvasObject{applicationSelect}, content.Objects...)
	}

	return content
}

//go:embed assets/fullscreen.svg
var fullscreenSVGBytes []byte

//go:embed assets/halfleft.svg
var halfLeftSVGBytes []byte

//go:embed assets/halfright.svg
var halfRightSVGBytes []byte

func makeAppSettingWindow(settings *Settings, appSetting AppSetting, isNew bool, parent fyne.Window, onClose func(newSetting *AppSetting)) *dialog.CustomDialog {
	currentWindowsMutex.Lock()
	windowsForSelect := getWindowsForSelect(currentWindows)
	currentWindowsMutex.Unlock()

	var appSettingDialog *dialog.CustomDialog
	var windowSub rx.Subscription

	leftIcon := fyne.NewStaticResource("right.svg", halfLeftSVGBytes)
	rightIcon := fyne.NewStaticResource("right.svg", halfRightSVGBytes)
	fullIcon := fyne.NewStaticResource("full.svg", fullscreenSVGBytes)

	monitorIdx := getDefaultMonitorIndex(settings, appSetting, isNew)
	selectedMonitor := monitors[monitorIdx]

	halfLeftBtn = createSizeButton(selectedMonitor, 2, 0, "Half Left", leftIcon)
	halfRightBtn = createSizeButton(selectedMonitor, 2, 1, "Half Right", rightIcon)
	fullBtn = createSizeButton(selectedMonitor, 1, 0, "Full", fullIcon)

	confirmButton = widget.NewButtonWithIcon("Create", theme.ConfirmIcon(), func() {
		if isNew {
			windowSub.Unsubscribe()
		}
		appSettingDialog.Hide()
		onClose(&appSetting)
	})
	confirmButton.Importance = widget.HighImportance
	confirmButton.Disable()

	if !isNew {
		confirmButton.SetText("Save")
	}

	cancelButton := widget.NewButtonWithIcon("Cancel", theme.CancelIcon(), func() {
		if isNew {
			windowSub.Unsubscribe()
		}
		appSettingDialog.Hide()
		onClose(nil)
	})

	applicationSelect = createApplicationSelect(windowsForSelect, &appSetting, isNew)
	displaySelect = createDisplaySelect(settings, &appSetting, isNew)
	matchType = createMatchTypeRadio(settings, &appSetting, isNew)

	windowSub = subscribeToWindowUpdates(windowsForSelect, isNew)

	content := createPresetsContent(settings, &appSetting, isNew, cancelButton, confirmButton)

	dialogName := "New App Config"
	if !isNew {
		dialogName = appSetting.Display()
	}
	appSettingDialog = dialog.NewCustomWithoutButtons(dialogName, content, parent)
	return appSettingDialog
}
