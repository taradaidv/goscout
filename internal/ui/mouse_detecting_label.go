package ui

import (
	"fmt"
	"io"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"
)

type MouseDetectingLabel struct {
	widget.Label
	path      string
	isBranch  bool
	entryFile *widget.Entry
	entryText *customMultiLineEntry
	ui        *UI
}

func NewMouseDetectingLabel(ui *UI, isBranch bool, entryFile *widget.Entry, entryText *customMultiLineEntry) *MouseDetectingLabel {
	label := &MouseDetectingLabel{
		Label:     *widget.NewLabel(""),
		path:      "",
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
		m.entryFile.SetText(m.path)
		if m.isBranch {
			m.entryFile.OnSubmitted(m.path)
		} else {
			m.handleSelection()
		}

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

func (m *MouseDetectingLabel) handleSelection() {
	selectedTabIndex := m.ui.fyneTabs.SelectedIndex()
	go func() {
		fileInfo, err := m.ui.activeSFTP[selectedTabIndex].Stat(m.path)
		if err != nil {
			m.ui.notifyError(fmt.Sprintf("Failed to get file info: %v", err))
			return
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

			m.ui.fyneWindow.Content().Refresh()
			m.entryText.SetText(m.entryFile.Text + "\nFile too large to display, " + sizeStr)
			m.entryText.TextStyle = fyne.TextStyle{Bold: true, Italic: true}
			m.entryText.Refresh()
			return
		}

		file, err := m.ui.activeSFTP[selectedTabIndex].Open(m.entryFile.Text)
		if err != nil {
			m.ui.notifyError(fmt.Sprintf("Failed to open file: %v", err))
			return
		}
		defer file.Close()

		content, err := io.ReadAll(file)
		if err != nil {
			m.ui.notifyError(fmt.Sprintf("Failed to read file: %v", err))
			return
		}

		m.ui.fyneWindow.Content().Refresh()
		if isReadable(content) {
			m.entryText.SetText(string(content))
		} else {
			m.entryText.SetText(m.entryFile.Text + "\nFile contains unreadable symbols")
			m.entryText.TextStyle = fyne.TextStyle{Bold: true, Italic: true}
			m.entryText.Refresh()
		}
	}()
}
