package ui

import (
	"fmt"
	"io"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"
)

func NewMouseDetectingLabel(ui *UI, isBranch bool, entryFile *widget.Entry, entryText *customMultiLineEntry) *MouseDetectingLabel {
	label := &MouseDetectingLabel{
		Label:     *widget.NewLabel(""),
		isBranch:  isBranch,
		entryFile: entryFile,
		entryText: entryText,
		ui:        ui,
	}
	label.ExtendBaseWidget(label)
	return label
}

func (m *MouseDetectingLabel) MouseUp(e *desktop.MouseEvent) {}

func (m *MouseDetectingLabel) MouseDown(e *desktop.MouseEvent) {
	switch e.Button {
	case desktop.MouseButtonPrimary:
		go m.entryFile.OnSubmitted(m.fullPath)
	case desktop.MouseButtonSecondary:
		m.showContextMenu(e)
	}
}
func (m *MouseDetectingLabel) showContextMenu(e *desktop.MouseEvent) {
	if m.isBranch {
		menu := fyne.NewMenu("",
			fyne.NewMenuItem("TODO:More folder actions ...", func() {
			}),
		)
		popUpMenu := widget.NewPopUpMenu(menu, m.ui.fyneWindow.Canvas())
		popUpMenu.ShowAtPosition(e.AbsolutePosition)
	} else {
		menu := fyne.NewMenu("",
			fyne.NewMenuItem("TODO:More file actions ...", func() {
			}),
		)
		popUpMenu := widget.NewPopUpMenu(menu, m.ui.fyneWindow.Canvas())
		popUpMenu.ShowAtPosition(e.AbsolutePosition)
	}
}

func (ui *UI) handleSelection(fullPath string) *customMultiLineEntry {
	selectedTabIndex := ui.fyneTabs.SelectedIndex()
	entryText := &customMultiLineEntry{}

	fileInfo, err := ui.activeSFTP[selectedTabIndex].Stat(fullPath)
	if err != nil {
		entryText.SetText(fullPath + "\n" + fmt.Sprintf("Failed to get file info: %v", err))
		entryText.TextStyle = fyne.TextStyle{Bold: true, Italic: true}
		return entryText
	}

	if fileInfo.Size() > 10*1024 {
		var sizeStr string
		const unit = 1024
		if fileInfo.Size() < unit {
			sizeStr = fmt.Sprintf("%d B", fileInfo.Size())
		} else {
			div, exp := int64(unit), 0
			for n := fileInfo.Size() / unit; n >= unit; n /= unit {
				div *= unit
				exp++
			}

			sizeStr = fmt.Sprintf("%.1f %cB", float64(fileInfo.Size())/float64(div), "KMGTPE"[exp])
		}

		ui.fyneWindow.Content().Refresh()

		entryText.SetText(fullPath + "\nFile too large to display, " + sizeStr)
		entryText.TextStyle = fyne.TextStyle{Bold: true, Italic: true}
		return entryText
	}

	file, err := ui.activeSFTP[selectedTabIndex].Open(fullPath)
	if err != nil {
		entryText.SetText(fullPath + "\n" + fmt.Sprintf("Failed to open file: %v", err))
		entryText.TextStyle = fyne.TextStyle{Bold: true, Italic: true}
		return entryText
	}
	defer file.Close()

	content, err := io.ReadAll(file)
	if err != nil {
		ui.notifyError(fmt.Sprintf("Failed to read file: %v", err))
		return entryText
	}

	ui.fyneWindow.Content().Refresh()
	if isReadable(content) {
		entryText.SetText(string(content))
		entryText.TextStyle = fyne.TextStyle{}
	} else {
		entryText.SetText(fullPath + "\nFile contains unreadable symbols")
		entryText.TextStyle = fyne.TextStyle{Bold: true, Italic: true}
	}
	return entryText
}
