package ui

import (
	"fmt"
	"image/png"
	"io"
	"os"
	"path"
	"strings"

	"goscout/internal/scoutssh"
	"goscout/internal/webdav"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/pkg/sftp"
)

const (
	repo       = "taradaidv/goscout"
	ver        = "v0.3.1"
	configFile = ".goscout.json"
)

func (ui *UI) SetHosts() {
	hosts, err := scoutssh.GetSSHHosts()
	if err != nil {

		dialog.ShowError(err, ui.fyneWindow)
		ui.fyneWindow.Close()
	}
	hosts = append(hosts, "[edit ➜ "+scoutssh.LocalHome+"/.ssh/config")
	ui.fyneSelect.Options = hosts
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
		ItemStore:        map[string]*TreeObject{},
		sshConfigEditor:  nil,
		logsLabel:        widget.NewMultiLineEntry(),
		connectionTab:    &container.TabItem{},
		bottomConnection: &fyne.Container{},
		webdavActive:     false, // Initialize the field
	}

	defer ui.fyneWindow.Close()
	ui.fyneWindow.SetMainMenu(fyne.NewMainMenu())

	ui.fyneWindow.Resize(fyne.NewSize(ui.cfg.WindowWidth, ui.cfg.WindowHeight))
	ui.fyneWindow.CenterOnScreen()

	ui.fyneSelect.OnChanged = func(selected string) {
		go ui.connectToHost(selected)
	}
	ui.fyneSelect.PlaceHolder = "lineup of available hosts"
	ui.connectionTab = container.NewTabItem("Hosts", nil)
	ui.connectionTab.Icon = theme.ComputerIcon()

	if len(ui.fyneTabs.Items) == 0 {
		go ui.SetHosts()
		go ui.setBottom()
		ui.logsLabel.Disabled()
		ui.connectionTab.Content = container.NewBorder(container.NewVBox(ui.fyneSelect), nil, nil, nil, container.NewVScroll(ui.logsLabel))
		ui.fyneTabs.Append(ui.connectionTab)

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
	ui.log(host, "connection...")
	sftpClient, sshClient, err := scoutssh.Connect(ui.fyneWindow, host)
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

	ui.fyneSelect.PlaceHolder = "lineup of available hosts"
	terminal, err := ui.setupSSHSession(host, sshClient)
	if err != nil {
		ui.log(host, err.Error())
		return nil
	}

	ui.activeSFTP[len(ui.fyneTabs.Items)] = sftpClient
	ui.log(host, "connected")

	treeData, err := scoutssh.FetchSFTPData(sftpClient, scoutssh.RemoteHome)
	if err != nil {
		sshClient.Close()
		sftpClient.Close()
		return nil
	}

	params := UIParams{
		Terminal: terminal,
		TreeData: treeData,
		data: &CustomEntry{
			Entry:      widget.Entry{},
			path:       &widget.Entry{},
			sftpClient: sftpClient,
		},
	}
	params.data.Entry.MultiLine = true
	params.data.Entry.ExtendBaseWidget(params.data)
	var split *container.Split
	params.data.path.OnSubmitted = func(fullPath string) {
		params.data.path.SetText(fullPath)
		if strings.HasSuffix(fullPath, "/") {
			treeData, err := scoutssh.FetchSFTPData(ui.activeSFTP[ui.fyneTabs.SelectedIndex()], fullPath)
			if err != nil {
				ui.notifyError(fmt.Sprintf("Failed to list files: %v", err))
				return
			}
			params.TreeData = treeData

			split = container.NewHSplit(ui.components(params))
			split.SetOffset(ui.cfg.SplitOffsets[host])

			ui.fyneTabs.Items[ui.fyneTabs.SelectedIndex()].Content = container.NewBorder(nil, nil, nil, nil, split)
			ui.fyneTabs.Refresh()
		} else {
			newEntryText := ui.handleSelection(fullPath)
			params.data.SetText(newEntryText.Text)
			params.data.TextStyle = newEntryText.TextStyle
			params.data.Refresh()
		}
	}

	params.data.path.SetText(scoutssh.RemoteHome)

	split = container.NewHSplit(ui.components(params))
	split.SetOffset(ui.cfg.SplitOffsets[host])

	remoteTab := container.NewTabItem(host, container.NewBorder(nil, nil, nil, nil, split))
	ui.fyneTabs.Append(remoteTab)
	ui.openTabs = append(ui.openTabs, host)
	return remoteTab
}

func trimPath(path string) string {
	if strings.HasSuffix(path, "/") {
		return path
	}
	lastSlashIndex := strings.LastIndex(path, "/")
	if lastSlashIndex != -1 {
		return path[:lastSlashIndex+1]
	}
	return path
}

func getPreviousDirectory(path string) string {
	path = trimPath(path)
	path = strings.TrimSuffix(path, "/")

	lastSlashIndex := strings.LastIndex(path, "/")
	if lastSlashIndex == -1 {
		return "/"
	}

	return path[:lastSlashIndex+1]
}

func (ui *UI) stopWebDAV() {
	if ui.webdavListener != nil {
		ui.webdavListener.Close()
		ui.webdavListener = nil
		ui.webdavActive = false
	}
}

func (ui *UI) setBottom() {

	var (
		fyneImg *canvas.Image
		banner  *widget.Label
	)
	resp, _ := fetchResponseBody("raw.githubusercontent.com/" + repo + "/main/docs/images/TON.png")
	img, err := png.Decode(resp.Body)

	if err == nil {
		fyneImg = canvas.NewImageFromImage(img)
		fyneImg.FillMode = canvas.ImageFillContain
		fyneImg.SetMinSize(fyne.NewSize(72, 72))
	} else {
		banner = widget.NewLabelWithStyle("GoScout ❤️s you", fyne.TextAlignCenter, fyne.TextStyle{Bold: true})
		ui.logsLabel.Wrapping = fyne.TextWrapWord
	}

	ui.logsLabel.Wrapping = fyne.TextWrapWord
	leftBottomContainer := container.NewBorder(
		nil,
		nil,
		nil,
		ui.SetVersion(),
	)

	if err == nil {
		ui.bottomConnection = container.NewBorder(nil, nil, leftBottomContainer, fyneImg)
	} else {
		ui.bottomConnection = container.NewBorder(nil, nil, leftBottomContainer, banner)
	}
	ui.connectionTab.Content = container.NewBorder(container.NewVBox(ui.fyneSelect), ui.bottomConnection, nil, nil, container.NewVScroll(ui.logsLabel))
}
func (ui *UI) components(params UIParams) (fyne.CanvasObject, fyne.CanvasObject) {
	rootButton := widget.NewButton("  /  ", func() {
		params.data.path.OnSubmitted("/")
	})

	var webdavButton *widget.Button
	webdavButton = widget.NewButton("Start WebDAV", func() {
		if ui.webdavActive {
			ui.stopWebDAV()
			webdavButton.SetText("Start WebDAV")
		} else {
			entryPoint, listener := webdav.Mount(ui.activeSFTP[ui.fyneTabs.SelectedIndex()])
			ui.webdavListener = listener
			content := container.NewVBox(
				widget.NewLabel("This is an experimental feature that starts WebDAV on the localhost without creating local folders,\nmeaning the file system is in-memory and available as long as GoScout is running."),
				widget.NewLabel("Example for macOS:\nHit CMD+K in Finder, enter the address below with ANY creds."),
				widget.NewHyperlink("http://"+entryPoint, parseURL("http://"+entryPoint)),
				layout.NewSpacer(),
			)
			dialog.ShowCustom("Experimental GoScout - WebDAV in memory", "OK", content, ui.fyneWindow)
			webdavButton.SetText("Stop WebDAV")
			ui.webdavActive = true
		}
	})

	toolbar := widget.NewToolbar(
		widget.NewToolbarAction(theme.HomeIcon(), func() {
			params.data.path.OnSubmitted(scoutssh.RemoteHome)
		}),
		widget.NewToolbarAction(theme.MoveUpIcon(), func() {
			path := getPreviousDirectory((params.data.path.Text))
			params.data.path.SetText(path)
			params.data.path.OnSubmitted(path)
		}),
		widget.NewToolbarAction(theme.DownloadIcon(), func() {
			fileSaveDialog := dialog.NewFolderOpen(func(list fyne.ListableURI, err error) {
				if err != nil {
					dialog.ShowError(err, ui.fyneWindow)
					return
				}
				if list == nil {
					return
				}

				localBasePath := list.Path()

				err = downloadFileOrDirectory(ui.activeSFTP[ui.fyneTabs.SelectedIndex()], params.data.path.Text, localBasePath)
				if err != nil {
					dialog.ShowError(err, ui.fyneWindow)
				} else {
					dialog.ShowInformation("Success", params.data.path.Text+"\nSaved in "+localBasePath, ui.fyneWindow)
				}
			}, ui.fyneWindow)

			screenSize := ui.fyneWindow.Canvas().Size()
			fileSaveDialog.Resize(screenSize)
			fileSaveDialog.Show()
		}),
	)

	toolbarContainer := container.NewHBox(
		rootButton,
		toolbar,
		webdavButton,
	)

	leftContent := container.NewBorder(
		toolbarContainer, nil, nil, nil,
		container.NewVScroll(ui.createList(params.TreeData, params.data.path, params.data)),
	)

	overlay := NewClickInterceptor(ui, params.Terminal)
	overlay.Resize(params.Terminal.Size())

	termWithOverlay := container.NewStack(params.Terminal, overlay)

	term := container.NewVSplit(
		container.NewVScroll(params.data),
		termWithOverlay,
	)

	rightContent := container.NewBorder(
		params.data.path, nil, nil, nil,
		term,
	)

	return leftContent, rightContent
}

func isDirectory(path string) bool {
	return strings.HasSuffix(path, "/")
}

func downloadFileOrDirectory(client *sftp.Client, remotePath, localBasePath string) error {
	if isDirectory(remotePath) {
		localDirPath := path.Join(localBasePath, path.Base(remotePath))
		return downloadDirectory(client, remotePath, localDirPath)
	} else {
		localFilePath := path.Join(localBasePath, path.Base(remotePath))
		return downloadFile(client, remotePath, localFilePath)
	}
}

func downloadDirectory(client *sftp.Client, remotePath, localBasePath string) error {
	entries, err := client.ReadDir(remotePath)
	if err != nil {
		return fmt.Errorf("failed to read directory: %v", err)
	}

	err = os.MkdirAll(localBasePath, os.ModePerm)
	if err != nil {
		return fmt.Errorf("failed to create local directory: %v", err)
	}

	for _, entry := range entries {
		entryRemotePath := path.Join(remotePath, entry.Name())
		entryLocalPath := path.Join(localBasePath, entry.Name())

		if entry.IsDir() {
			err = downloadDirectory(client, entryRemotePath, entryLocalPath)
		} else {
			err = downloadFile(client, entryRemotePath, entryLocalPath)
		}
		if err != nil {
			return err
		}
	}

	return nil
}
func downloadFile(client *sftp.Client, remotePath, localPath string) error {
	srcFile, err := client.Open(remotePath)
	if err != nil {
		return fmt.Errorf("failed to open remote file: %v", err)
	}
	defer srcFile.Close()

	dstFile, err := os.Create(localPath)
	if err != nil {
		return fmt.Errorf("failed to create local file: %v", err)
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	if err != nil {
		return fmt.Errorf("failed to copy file: %v", err)
	}

	return nil
}

func uploadDirectory(client *sftp.Client, localPath, remoteBasePath string) error {
	entries, err := os.ReadDir(localPath)
	if err != nil {
		return fmt.Errorf("failed to read local directory: %v", err)
	}

	err = client.MkdirAll(remoteBasePath)
	if err != nil {
		return fmt.Errorf("failed to create remote directory: %v", err)
	}

	for _, entry := range entries {
		entryLocalPath := path.Join(localPath, entry.Name())
		entryRemotePath := path.Join(remoteBasePath, entry.Name())

		if entry.IsDir() {
			err = uploadDirectory(client, entryLocalPath, entryRemotePath)
		} else {
			err = uploadFile(client, entryLocalPath, entryRemotePath)
		}
		if err != nil {
			return err
		}
	}

	return nil
}

func uploadFile(client *sftp.Client, localPath, remotePath string) error {
	srcFile, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("failed to open local file: %v", err)
	}
	defer srcFile.Close()

	dstFile, err := client.Create(remotePath)
	if err != nil {
		return fmt.Errorf("failed to create remote file: %v", err)
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	if err != nil {
		return fmt.Errorf("failed to copy file: %v", err)
	}

	return nil
}
