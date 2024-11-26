package ui

import (
	"encoding/json"
	"fmt"
	"goscout/internal/scoutssh"
	"io"
	"net/http"
	"net/url"
	"time"
	"unicode/utf8"

	"image/color"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	"github.com/fyne-io/terminal"
	"github.com/pkg/errors"
	"golang.org/x/crypto/ssh"
)

func getConfigFilePath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", errors.Wrap(err, "getting user home directory")
	}
	return filepath.Join(homeDir, configFile), nil
}

func LoadConfig() (*Config, error) {
	defaultConfig := &Config{
		WindowWidth:  800.0,
		WindowHeight: 600.0,
		SplitOffset:  0.3,
		OpenTabs:     []string{},
	}

	configFilePath, err := getConfigFilePath()
	if err != nil {
		return nil, err
	}

	file, err := os.Open(configFilePath)
	if err != nil {
		if os.IsNotExist(err) {
			return defaultConfig, nil
		}
		return nil, errors.New("opening config file")
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	err = decoder.Decode(defaultConfig)
	if err != nil {
		if errors.Is(err, os.ErrInvalid) || err.Error() == "EOF" {
			return defaultConfig, nil
		}
		return nil, errors.New("decoding config file")
	}

	return defaultConfig, nil
}

func SaveConfig(config *Config) error {
	configFilePath, err := getConfigFilePath()
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return errors.Wrap(err, "marshalling config")
	}

	return errors.Wrap(os.WriteFile(configFilePath, data, 0644), "writing config file")
}

func (ui *UI) saveFile() {
	selectedTabIndex := ui.fyneTabs.SelectedIndex()
	entryText := ui.entryTexts[selectedTabIndex]
	entryFile := ui.entryFiles[selectedTabIndex]

	if entryText == nil || entryFile == nil {
		ui.notifyError("No entry text or file path found for the active tab")
		return
	}

	sftpClient := ui.activeSFTP[selectedTabIndex]
	filePath := entryFile.Text

	file, err := sftpClient.OpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC)
	if err != nil {
		ui.notifyError(fmt.Sprintf("Failed to open file: %v", err))
		return
	}
	defer file.Close()

	content := entryText.Text
	_, err = file.Write([]byte(content))
	if err != nil {
		ui.notifyError(fmt.Sprintf("Failed to write to file: %v", err))
		return
	}

	err = file.Close()
	if err != nil {
		ui.notifyError(fmt.Sprintf("Failed to close file: %v", err))
		return
	}
	entryFile.OnSubmitted(entryFile.Text)
	ui.notifySuccess("File saved successfully")
}

func (ui *UI) setupSSHSession(host string, sshClient *ssh.Client) (*ssh.Session, io.WriteCloser, io.Reader, *terminal.Terminal, error) {
	session, err := sshClient.NewSession()
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("failed to create session: %w", err)
	}

	if err := session.RequestPty("xterm", 80, 40, ssh.TerminalModes{}); err != nil {
		return nil, nil, nil, nil, fmt.Errorf("failed to request PTY: %w", err)
	}

	in, err := session.StdinPipe()
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("failed to get stdin pipe: %w", err)
	}

	out, err := session.StdoutPipe()
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("failed to get stdout pipe: %w", err)
	}

	go func() {
		if err := session.Shell(); err != nil {
			ui.log(host, err.Error())
		}
	}()

	t := terminal.New()
	go func() {
		if err := t.RunWithConnection(in, out); err != nil {
			ui.log(host, err.Error())
		}
	}()

	ch := make(chan terminal.Config, 1)
	go func() {
		var rows, cols uint
		for config := range ch {
			if rows != config.Rows || cols != config.Columns {
				rows, cols = config.Rows, config.Columns
				session.WindowChange(int(rows), int(cols))
			}
		}
	}()
	t.AddListener(ch)

	return session, in, out, t, nil
}

func (ui *UI) createList(remoteTree map[string][]scoutssh.FileInfo, entryFile *widget.Entry, entryText *customMultiLineEntry) *widget.List {
	uniqueItems := make(map[string]bool)
	for key, children := range remoteTree {
		uniqueItems[key] = true
		for _, child := range children {
			var childPath string
			if key == "" {
				childPath = child.Name
			} else {
				childPath = strings.TrimSuffix(key, "/") + "/" + child.Name
			}
			uniqueItems[childPath] = child.IsDir
			ui.ItemStore[childPath] = &TreeObject{FileInfo: child}
		}
	}

	var items []string
	for item := range uniqueItems {
		items = append(items, item)
	}
	sort.Strings(items)
	entryText.TextStyle = fyne.TextStyle{Bold: false, Italic: false}

	list := widget.NewList(
		func() int {
			if len(items) > 0 {
				return len(items) - 1
			}
			return 0
		},
		func() fyne.CanvasObject {
			return NewMouseDetectingLabel(ui, false, entryFile, entryText)
		},
		func(i widget.ListItemID, obj fyne.CanvasObject) {
			uid := items[i+1]
			isBranch := uniqueItems[uid]

			node := obj.(*MouseDetectingLabel)
			node.path = uid
			segments := strings.Split(uid, "/")
			node.SetText(segments[len(segments)-1])
			node.isBranch = isBranch
			if isBranch {
				node.TextStyle.Bold = true
				node.Wrapping = fyne.TextWrapWord
			} else {
				node.TextStyle.Bold = false
			}

			if treeObject, exists := ui.ItemStore[uid]; exists {
				info := treeObject.FileInfo
				if info.IsLink {
					node.SetText(info.Name + "*")
				}
			}
		},
	)

	list.OnSelected = func(id widget.ListItemID) {
		uid := items[id]
		entryFile.OnSubmitted(uid)
	}
	return list
}

func (ui *UI) saveState() {
	ui.cfg.WindowWidth = ui.fyneWindow.Canvas().Size().Width
	ui.cfg.WindowHeight = ui.fyneWindow.Canvas().Size().Height
	ui.cfg.OpenTabs = []string{}
	for _, tab := range ui.fyneTabs.Items {
		if tab.Icon == nil {
			ui.cfg.OpenTabs = append(ui.cfg.OpenTabs, tab.Text)
		}
	}

	err := SaveConfig(ui.cfg)
	if err != nil {
		log.Printf("Failed to save config: %v", err)
	}
}

func isReadable(content []byte) bool {
	for len(content) > 0 {
		r, size := utf8.DecodeRune(content)
		if r == utf8.RuneError && size == 1 {
			return false
		}
		if !isPrintable(r) {
			return false
		}
		content = content[size:]
	}
	return true
}

func isPrintable(r rune) bool {
	return r == '\t' || r == '\n' || r == '\r' || (r >= ' ' && r <= '~') || (r > 127 && r != utf8.RuneError)
}

func actionSSHconfig(ui *UI, cfgFile string) *saveSSHconfig {
	entry := &saveSSHconfig{
		cfgFile: cfgFile,
		ui:      ui,
	}
	entry.ExtendBaseWidget(entry)
	entry.MultiLine = true
	return entry
}

func (e *saveSSHconfig) TypedShortcut(shortcut fyne.Shortcut) {
	if sc, ok := shortcut.(*desktop.CustomShortcut); ok {
		if sc.KeyName == fyne.KeyS && (sc.Modifier == fyne.KeyModifierControl || sc.Modifier == fyne.KeyModifierSuper) {
			file, err := os.OpenFile(e.cfgFile, os.O_TRUNC|os.O_WRONLY, 0)
			if err != nil {
				log.Fatal(err)
				return
			}

			_, err = file.WriteString(e.Text)
			if err != nil {
				log.Fatal(err)
				return
			}
			file.Close()
			e.ui.SetHosts()
			e.ui.ToggleContent()

			content := container.NewVBox(
				widget.NewLabel("✅ The SSH client configuration file has been saved."),
				widget.NewLabel("Stay trendy, move away from passwords."),
				layout.NewSpacer(),
			)
			dialog.ShowCustom("SSH config", "OK", content, e.ui.fyneWindow)

		}
	}
	e.Entry.TypedShortcut(shortcut)
}

func (ui *UI) ToggleContent() {

	if ui.sshConfigEditor == nil {
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
		ui.connectionTab.Content = container.NewBorder(container.NewVBox(ui.fyneSelect), nil, nil, nil, container.NewVScroll(ui.sshConfigEditor))
	} else {
		ui.sshConfigEditor = nil
		ui.connectionTab.Content = container.NewBorder(container.NewVBox(ui.fyneSelect), ui.bottomConnection, nil, nil, container.NewVScroll(ui.logsLabel))
	}

}

func fetchResponseBody(webEntry string) (*http.Response, error) {
	link := parseURL("https://" + webEntry)
	httpClient := &http.Client{
		Timeout: 5 * time.Second,
	}

	req, err := http.NewRequest("GET", link.String(), nil)
	if err == nil {
		req.Header.Set("User-Agent", "GoScout")
	}
	resp, err := httpClient.Do(req)

	return resp, err
}

func processTags(body io.ReadCloser) (string, error) {
	defer body.Close()

	var tags []Tag
	if err := json.NewDecoder(body).Decode(&tags); err != nil {
		return "", err
	}

	if len(tags) == 0 {
		return "", fmt.Errorf("no tags found")
	}

	return tags[0].Name, nil
}

func parseURL(urlStr string) *url.URL {
	parsedURL, _ := url.Parse(urlStr)
	return parsedURL
}

func (ui *UI) SetVersion() *widget.RichText {
	resp, _ := fetchResponseBody("api.github.com/repos/" + repo + "/tags")
	defer resp.Body.Close()
	var tagLabel *widget.RichText
	tag, _ := processTags(resp.Body)
	if tag == "" {
		tag = "on-prem mode"
	}
	if tag != ver && tag != "on-prem mode" && tag != "" {

		tagLabel = widget.NewRichText(
			&widget.TextSegment{Text: "current version: " + ver + " / main version: " + tag},
			&widget.HyperlinkSegment{Text: repo, URL: parseURL("https://github.com/" + repo)},
		)
	} else {
		tagLabel = widget.NewRichText(
			&widget.TextSegment{Text: tag},
			&widget.HyperlinkSegment{Text: repo, URL: parseURL("https://github.com/" + repo)},
		)
	}
	return tagLabel
}

func newCustomMultiLineEntry(ui *UI) *customMultiLineEntry {
	entry := &customMultiLineEntry{ui: ui}
	entry.ExtendBaseWidget(entry)
	entry.MultiLine = true
	return entry
}

func (e *customMultiLineEntry) TypedShortcut(shortcut fyne.Shortcut) {
	if sc, ok := shortcut.(*desktop.CustomShortcut); ok {
		if sc.KeyName == fyne.KeyS && (sc.Modifier == fyne.KeyModifierControl || sc.Modifier == fyne.KeyModifierSuper) {
			e.ui.saveFile()
			return
		}
	}
	e.Entry.TypedShortcut(shortcut)
}

func NewClickInterceptor(ui *UI, t *terminal.Terminal) *ClickInterceptor {
	ci := &ClickInterceptor{ui: ui, t: t}
	ci.ExtendBaseWidget(ci)
	return ci
}

func (ci *ClickInterceptor) CreateRenderer() fyne.WidgetRenderer {
	rect := canvas.NewRectangle(color.Transparent)
	return &clickInterceptorRenderer{rect: rect}
}

// Invisible area click for boost terminal swith
func (ci *ClickInterceptor) Tapped(*fyne.PointEvent) {
	ci.ui.fyneWindow.Canvas().Focus(ci.t)
}

func (r *clickInterceptorRenderer) Layout(size fyne.Size) {
	r.rect.Resize(size)
}

func (r *clickInterceptorRenderer) MinSize() fyne.Size {
	return r.rect.MinSize()
}

func (r *clickInterceptorRenderer) Refresh() {
	canvas.Refresh(r.rect)
}

func (r *clickInterceptorRenderer) BackgroundColor() color.Color {
	return color.Transparent
}

func (r *clickInterceptorRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.rect}
}

func (r *clickInterceptorRenderer) Destroy() {}
