package ui

import "fyne.io/fyne/v2"

func (ui *UI) notifyError(message string) {
	fyne.CurrentApp().SendNotification(&fyne.Notification{
		Title:   "Error",
		Content: message,
	})
}

func (ui *UI) notifySuccess(message string) {
	fyne.CurrentApp().SendNotification(&fyne.Notification{
		Title:   "Success",
		Content: message,
	})
}

func (ui *UI) log(host, message string) {
	//dialog.ShowError(err, ui.fyneWindow)
	if ui.logsLabel.Text != "" {
		ui.logsLabel.SetText(ui.logsLabel.Text + "\n" + host + ": " + message)
	} else {
		ui.logsLabel.SetText(host + ": " + message)
	}

}
