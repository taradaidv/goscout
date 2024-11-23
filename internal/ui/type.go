package ui

import (
	"goscout/internal/scoutssh"

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
	list             *widget.List
	ItemStore        map[string]*TreeObject
	sideLabels       map[*container.TabItem]*widget.Label
	entryTexts       map[int]*customMultiLineEntry
	entryFiles       map[int]*widget.Entry
	sshConfigEditor  *saveSSHconfig
	contentContainer *fyne.Container
	fyneImg          *canvas.Image
	label            *widget.Label
	tagLabel         *widget.RichText
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

type customMultiLineEntry struct {
	widget.Entry
	ui *UI
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
	WindowWidth  float32 `json:"window_width"`
	WindowHeight float32 `json:"window_height"`
	SplitOffset  float64 `json:"split_offset"`
	OpenTabs     []string
}

type MouseDetectingLabel struct {
	widget.Label
	path      string
	isBranch  bool
	entryFile *widget.Entry
	entryText *customMultiLineEntry
	ui        *UI
}
