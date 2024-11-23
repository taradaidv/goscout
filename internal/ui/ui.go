package ui

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	"goscout/internal/scoutssh"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/pkg/sftp"
)

const (
	repo       = "taradaidv/goscout"
	ver        = "v0.2.0-alpha"
	configFile = ".goscout.json"
)

func (ui *UI) updateHosts() {
	hosts, err := scoutssh.GetSSHHosts()
	if err != nil {

		dialog.ShowError(err, ui.fyneWindow)
		ui.fyneWindow.Close()
	}
	hosts = append(hosts, "[edit ➜ ~/.ssh/config]")
	ui.fyneSelect.Options = hosts
	ui.fyneSelect.PlaceHolder = "lineup of available hosts"
	ui.fyneSelect.Refresh()
}

func SetupWindow(fyneWindow fyne.Window, cfg *Config) {
	ui := &UI{
		fyneWindow:       fyneWindow,
		fyneSelect:       widget.NewSelect(nil, nil),
		fyneTabs:         &container.DocTabs{},
		cfg:              cfg,
		openTabs:         []string{},
		activeSFTP:       make(map[int]*sftp.Client),
		list:             &widget.List{},
		ItemStore:        map[string]*TreeObject{},
		sideLabels:       map[*container.TabItem]*widget.Label{},
		entryTexts:       map[int]*customMultiLineEntry{},
		entryFiles:       map[int]*widget.Entry{},
		sshConfigEditor:  &saveSSHconfig{},
		contentContainer: &fyne.Container{},
		fyneImg:          &canvas.Image{},
		label:            &widget.Label{},
		tagLabel:         &widget.RichText{},
		logsLabel:        &widget.Label{},
	}

	defer ui.fyneWindow.Close()
	ui.fyneWindow.SetMainMenu(fyne.NewMainMenu(
		fyne.NewMenu("GoScout",
			fyne.NewMenuItem("Exit", func() {
				ui.fyneWindow.Close()
			}),
		),
	))

	ui.fyneWindow.Resize(fyne.NewSize(ui.cfg.WindowWidth, ui.cfg.WindowHeight))
	ui.fyneWindow.CenterOnScreen()

	ui.fyneSelect.OnChanged = func(selected string) {
		go ui.connectToHost(selected)
	}

	connectionTab := container.NewTabItem("Hosts", nil)
	connectionTab.Icon = theme.ComputerIcon()

	if len(ui.fyneTabs.Items) == 0 {
		ui.updateHosts()
		file, err := os.Open(filepath.Join(os.Getenv("HOME"), ".ssh", "config"))
		if err != nil {
			log.Fatal(err)
		}
		defer file.Close()

		buffer := make([]byte, 1024)
		n, err := file.Read(buffer)
		if err != nil && err != io.EOF {
			log.Fatalln("EOF", err)
		}

		ui.sshConfigEditor = actionSSHconfig(ui, file.Name())
		ui.sshConfigEditor.SetText(string(buffer[:n]))

		connectionTab.Content = ui.CreateConnectionContent()
		ui.fyneTabs.Append(connectionTab)
	}

	ui.fyneWindow.SetContent(ui.fyneTabs)

	ui.fyneWindow.SetOnClosed(func() {
		ui.saveState()
	})

	ui.fyneTabs.OnClosed = func(tab *container.TabItem) {
		if tab.Icon == nil {
			ui.saveState()
		} else {
			ui.fyneWindow.Close()
		}
	}

	ui.fyneTabs.OnSelected = func(tab *container.TabItem) {}

	for _, host := range ui.cfg.OpenTabs {
		go ui.connectToHost(host)
	}

	if len(ui.fyneTabs.Items) > 0 {
		ui.fyneTabs.Select(ui.fyneTabs.Items[len(ui.fyneTabs.Items)-1])
	}
}

func (ui *UI) connectToHost(host string) *container.TabItem {
	ui.fyneSelect.Selected = ""
	if ui.fyneSelect.Options[len(ui.fyneSelect.Options)-1] == host {
		ui.ToggleContent()
		return nil
	}

	sftpClient, sshClient, treeData, err := scoutssh.ConnectAndListFiles(host, ".")
	if err != nil {
		ui.log(host, err.Error())
		return nil
	}

	if sftpClient == nil || sshClient == nil {
		ui.log(host, "client nil")
		return nil
	}

	defer func() {
		if err != nil {
			ui.log(host, err.Error())
			sftpClient.Close()
			sshClient.Close()
		}
	}()

	_, _, _, t, err := ui.setupSSHSession(host, sshClient)
	if err != nil {
		ui.log(host, err.Error())
		return nil
	}

	ui.activeSFTP[len(ui.fyneTabs.Items)] = sftpClient

	entryFile := widget.NewEntry()
	entryFile.SetPlaceHolder("entry path ...")

	entryText := newCustomMultiLineEntry(ui)

	ui.entryFiles[len(ui.fyneTabs.Items)] = entryFile
	ui.entryTexts[len(ui.fyneTabs.Items)] = entryText

	var split *container.Split
	entryFile.OnSubmitted = func(path string) {
		tabID := ui.fyneTabs.SelectedIndex()

		treeData, err := scoutssh.FetchSFTPData(ui.activeSFTP[tabID], path)
		if err != nil {
			ui.notifyError(fmt.Sprintf("Failed to list files: %v", err))
			return
		}
		params := UIParams{
			Terminal:  t,
			TreeData:  treeData,
			EntryFile: entryFile,
			EntryText: entryText,
		}

		split = container.NewHSplit(ui.components(params))
		split.SetOffset(ui.cfg.SplitOffset)

		ui.fyneTabs.Items[tabID].Content = container.NewBorder(nil, nil, nil, nil, split)
		ui.fyneTabs.Refresh()
	}

	params := UIParams{
		Terminal:  t,
		TreeData:  treeData,
		EntryFile: entryFile,
		EntryText: entryText,
	}

	split = container.NewHSplit(ui.components(params))
	split.SetOffset(ui.cfg.SplitOffset)

	remoteTab := container.NewTabItem(host, container.NewBorder(nil, nil, nil, nil, split))
	ui.fyneTabs.Append(remoteTab)
	ui.openTabs = append(ui.openTabs, host)
	return remoteTab
}

func getPreviousDirectory(path string) string {
	if path == "./" || path == "." {
		return "./"
	}

	if strings.HasSuffix(path, "/") {
		path = strings.TrimSuffix(path, "/")
	} else {
		lastSlashIndex := strings.LastIndex(path, "/")
		if lastSlashIndex != -1 {
			path = path[:lastSlashIndex]
		}
	}

	lastSlashIndex := strings.LastIndex(path, "/")
	if lastSlashIndex == -1 {
		return "/"
	}

	return path[:lastSlashIndex+1]
}

func (ui *UI) components(params UIParams) (fyne.CanvasObject, fyne.CanvasObject) {
	toolbar := widget.NewToolbar(
		widget.NewToolbarAction(theme.HomeIcon(), func() {
			params.EntryFile.OnSubmitted(".")
		}),
		widget.NewToolbarAction(theme.MenuIcon(), func() {
			params.EntryFile.OnSubmitted("/")
		}),
		widget.NewToolbarAction(theme.MoveUpIcon(), func() {
			path := getPreviousDirectory((params.EntryFile.Text))
			params.EntryFile.SetText(path)
			params.EntryFile.OnSubmitted(path)

		}),
	)

	ui.createList(params.TreeData, params.EntryFile, params.EntryText)

	leftContent := container.NewBorder(
		toolbar, nil, nil, nil,
		container.NewVScroll(ui.list),
	)

	overlay := NewClickInterceptor(ui, params.Terminal)
	overlay.Resize(params.Terminal.Size())

	termWithOverlay := container.NewStack(params.Terminal, overlay)

	term := container.NewVSplit(
		container.NewVScroll(params.EntryText),
		termWithOverlay,
	)

	rightContent := container.NewBorder(
		params.EntryFile, nil, nil, nil,
		term,
	)

	return leftContent, rightContent
}
