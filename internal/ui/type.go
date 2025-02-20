package ui

import (
	"goscout/internal/scoutssh"
	"net"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
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
	ItemStore        map[string]*TreeObject
	sshConfigEditor  *saveSSHconfig
	logsLabel        *widget.Entry
	connectionTab    *container.TabItem
	bottomConnection *fyne.Container
	webdavActive     bool
	webdavListener   net.Listener // Add this field
}

type UIParams struct {
	Terminal *terminal.Terminal
	TreeData map[string][]scoutssh.FileInfo
	data     *CustomEntry
}

type UIComponents struct {
	Toolbar         *widget.Toolbar
	LeftContent     fyne.CanvasObject
	RightContent    fyne.CanvasObject
	Overlay         *ClickInterceptor
	TermWithOverlay fyne.CanvasObject
	Term            fyne.CanvasObject
}

type CustomEntry struct {
	widget.Entry
	path       *widget.Entry
	sftpClient *sftp.Client
}

type saveSSHconfig struct {
	cfgFile string
	widget.Entry
	ui *UI
}

type Tag struct {
	Name string `json:"name"`
}

type ClickInterceptor struct {
	widget.BaseWidget
	ui *UI
	t  *terminal.Terminal
}

type clickInterceptorRenderer struct {
	rect *canvas.Rectangle
}

type Config struct {
	WindowWidth  float32            `json:"window_width"`
	WindowHeight float32            `json:"window_height"`
	SplitOffsets map[string]float64 `json:"split_offsets"`
	OpenTabs     []string           `json:"open_tabs"`
}

type MouseDetectingLabel struct {
	widget.Label
	fullPath  string
	isBranch  bool
	entryFile *widget.Entry
	entryText *CustomEntry
	ui        *UI
}
