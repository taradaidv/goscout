package ui

import (
	"fmt"
	"log"

	"goscout/internal/scoutssh"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/fyne-io/terminal"
	"github.com/pkg/sftp"
)

type TreeObject struct {
	HeaderText string
	Payload    string
	Notes      string
	FileInfo   scoutssh.FileInfo
}
type UI struct {
	fyneWindow       fyne.Window
	fyneSelect       *widget.Select
	fyneTabs         *container.DocTabs
	cfg              *Config
	openTabs         []string
	activeSFTP       map[int]*sftp.Client
	list             *widget.List
	ItemStore        map[string]*TreeObject
	sideLabels       map[*container.TabItem]*widget.Label
	entryTexts       map[int]*customMultiLineEntry
	entryFiles       map[int]*widget.Entry
	sshConfigEditor  *saveSSHconfig
	contentContainer *fyne.Container
	fyneImg          *canvas.Image
	label            *widget.Label
}

func (ui *UI) updateHosts() {
	hosts, cfgFile, err := scoutssh.GetSSHHosts()
	if err != nil {
		log.Fatal(err)
	}
	hosts = append(hosts, "[edit ➜ "+cfgFile.Name()+"]")
	ui.fyneSelect.Options = hosts
	ui.fyneSelect.PlaceHolder = "lineup of available hosts"
	cfgFile.Close()
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
	}

	defer ui.fyneWindow.Close()
	ui.fyneWindow.SetMainMenu(fyne.NewMainMenu(
		fyne.NewMenu("GoScout",
			fyne.NewMenuItem("Exit", func() {
				ui.fyneWindow.Close()
			}),
		),
	))

	ui.updateHosts()

	ui.fyneWindow.Resize(fyne.NewSize(ui.cfg.WindowWidth, ui.cfg.WindowHeight))
	ui.fyneWindow.CenterOnScreen()

	ui.fyneSelect.OnChanged = func(selected string) {
		ui.connectToHost(selected)
	}

	connectionTab := container.NewTabItem("Hosts", nil)
	connectionTab.Icon = theme.ComputerIcon()

	if len(ui.fyneTabs.Items) == 0 {
		_, cfgFile, err := scoutssh.GetSSHHosts()
		if err != nil {
			log.Fatal(err)
		}
		cfgFile.Seek(0, 0)
		buffer := make([]byte, 1024)
		defer cfgFile.Close()

		n, err := cfgFile.Read(buffer)
		if err != nil {
			if err.Error() != "EOF" {
				fmt.Println("Error reading file:", err)
			}
		}

		ui.sshConfigEditor = actionSSHconfig(ui, cfgFile.Name())
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
		ui.connectToHost(host)
	}

	if len(ui.fyneTabs.Items) > 0 {
		ui.fyneTabs.Select(ui.fyneTabs.Items[len(ui.fyneTabs.Items)-1])
	}
}

func (ui *UI) connectToHost(host string) *container.TabItem {

	if ui.fyneSelect.Options[len(ui.fyneSelect.Options)-1] == host {
		ui.ToggleContent()
		return nil
	}

	sftpClient, sshClient, treeData, err := scoutssh.ConnectAndListFiles(host, ".")
	if err != nil {
		dialog.ShowError(err, ui.fyneWindow)
		return nil
	}

	if sftpClient == nil || sshClient == nil {
		ui.notifyError("Failed to establish connection")
		return nil
	}

	defer func() {
		if err != nil {
			sftpClient.Close()
			sshClient.Close()
		}
	}()

	_, _, _, t, err := ui.setupSSHSession(sshClient)
	if err != nil {
		ui.notifyError(err.Error())
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

type UIParams struct {
	Terminal  *terminal.Terminal
	TreeData  map[string][]scoutssh.FileInfo
	EntryFile *widget.Entry
	EntryText *customMultiLineEntry
}

type UIComponents struct {
	Toolbar         *widget.Toolbar
	LeftContent     fyne.CanvasObject
	RightContent    fyne.CanvasObject
	Overlay         *ClickInterceptor
	TermWithOverlay fyne.CanvasObject
	Term            fyne.CanvasObject
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
			params.EntryFile.OnSubmitted(params.EntryFile.Text + "/..")
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
