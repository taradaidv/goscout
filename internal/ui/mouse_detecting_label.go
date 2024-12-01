package ui

import (
	"fmt"
	"goscout/internal/scoutssh"
	"io"
	"path"
	"path/filepath"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"
)

func NewMouseDetectingLabel(ui *UI, isBranch bool, entryFile *widget.Entry, entryText *CustomEntry) *MouseDetectingLabel {
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
	var menuItems []*fyne.MenuItem
	mainPath := trimPath((m.fullPath))
	if m.isBranch {
		menuItems = append(menuItems, fyne.NewMenuItem("Action in folder: "+m.fullPath, func() {
			// Branch-specific action logic
		}))
	} else {
		menuItems = append(menuItems, fyne.NewMenuItem("Action in folder: "+filepath.Dir(m.fullPath)+"/", func() {
			// Branch-specific action logic
		}))
	}

	// Common action for both folders and files
	menuItems = append(menuItems, fyne.NewMenuItem("Upload file", func() {
		fileOpenDialog := dialog.NewFileOpen(func(reader fyne.URIReadCloser, err error) {
			if err != nil {
				dialog.ShowError(err, m.ui.fyneWindow)
				return
			}
			if reader == nil {
				return
			}
			defer reader.Close()

			localPath := reader.URI().Path()
			remotePath := path.Join(mainPath, path.Base(localPath))

			err = uploadFile(m.ui.activeSFTP[m.ui.fyneTabs.SelectedIndex()], localPath, remotePath)
			if err != nil {
				dialog.ShowError(err, m.ui.fyneWindow)
			} else {
				dialog.ShowInformation("Success", "File uploaded successfully", m.ui.fyneWindow)
				go m.entryFile.OnSubmitted(mainPath)
			}
		}, m.ui.fyneWindow)

		screenSize := m.ui.fyneWindow.Canvas().Size()
		fileOpenDialog.Resize(screenSize)
		fileOpenDialog.Show()
	}))

	menuItems = append(menuItems, fyne.NewMenuItem("Upload folder", func() {
		folderOpenDialog := dialog.NewFolderOpen(func(list fyne.ListableURI, err error) {
			if err != nil {
				dialog.ShowError(err, m.ui.fyneWindow)
				return
			}
			if list == nil {
				return
			}

			localPath := list.Path()
			remotePath := path.Join(mainPath, path.Base(localPath))

			err = uploadDirectory(m.ui.activeSFTP[m.ui.fyneTabs.SelectedIndex()], localPath, remotePath)
			if err != nil {
				dialog.ShowError(err, m.ui.fyneWindow)
			} else {
				dialog.ShowInformation("Success", "Directory uploaded successfully", m.ui.fyneWindow)
				go m.entryFile.OnSubmitted(mainPath)
			}
		}, m.ui.fyneWindow)

		screenSize := m.ui.fyneWindow.Canvas().Size()
		folderOpenDialog.Resize(screenSize)
		folderOpenDialog.Show()
	}))

	menuItems = append(menuItems, fyne.NewMenuItemSeparator())
	menuItems = append(menuItems, fyne.NewMenuItem("🔴 remove: "+m.Text, func() {
		path, err := scoutssh.RemoveSFTP(m.ui.activeSFTP[m.ui.fyneTabs.SelectedIndex()], m.fullPath)
		if err == nil {
			go m.entryFile.OnSubmitted(path)
		}

	}))

	menu := fyne.NewMenu("", menuItems...)
	popUpMenu := widget.NewPopUpMenu(menu, m.ui.fyneWindow.Canvas())
	popUpMenu.ShowAtPosition(e.AbsolutePosition)
}

func (ui *UI) handleSelection(fullPath string) *widget.Entry {
	selectedTabIndex := ui.fyneTabs.SelectedIndex()
	entryText := &widget.Entry{}

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
