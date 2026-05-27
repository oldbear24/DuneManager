package ui

import (
	"fmt"
	"image/color"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"

	"github.com/oldbear24/DuneManager/internal/api"
	"github.com/oldbear24/DuneManager/internal/config"
	"github.com/oldbear24/DuneManager/internal/discord"
	"github.com/oldbear24/DuneManager/internal/runner"
	"github.com/oldbear24/DuneManager/internal/updater"
	"golang.org/x/sys/windows"
)

// ── global state ───────────────────────────────────────────────────────────────

var (
	fyneApp    fyne.App
	mainWindow fyne.Window
	svcClient  *api.Client

	// status bar widgets
	statusDot   *canvas.Circle
	statusLabel *widget.Label
	ipLabel     *widget.Label

	// output area
	outputEntry  *widget.Entry
	outputScroll *container.Scroll
	outputMu     sync.Mutex
	outputBuf    strings.Builder

	// kill button visible in output header
	killBtn *widget.Button

	// update button shown in status bar when a newer version is available
	updateBtn    *widget.Button
	updateGUIURL string // stored GUI download URL when update is ready

	// all command buttons — for bulk enable/disable
	allButtons []*widget.Button

	// VM section buttons
	btnStartVM    *widget.Button
	btnStopVM     *widget.Button
	btnRotateSSH  *widget.Button
	btnChangePass *widget.Button

	// Battlegroup section buttons
	btnBGStatus   *widget.Button
	btnBGStart    *widget.Button
	btnBGRestart  *widget.Button
	btnBGStop     *widget.Button
	btnBGUpdate   *widget.Button
	btnBGSwap     *widget.Button
	btnBGBackup   *widget.Button
	btnBGBrowser  *widget.Button
	btnBGDirector *widget.Button

	// latest service status
	currentStatus *api.StatusResponse
	statusMu      sync.RWMutex

	// true while a command is executing
	cmdRunning bool
	cmdMu      sync.Mutex
)

// ── entry point ────────────────────────────────────────────────────────────────

// Run creates and shows the main GUI window.
func Run() {
	svcClient = api.NewClient()

	fyneApp = app.New()
	fyneApp.SetIcon(appIcon())
	mainWindow = fyneApp.NewWindow("Dune Awakening — Server Manager")
	mainWindow.Resize(fyne.NewSize(960, 720))
	mainWindow.SetMaster()
	mainWindow.SetContent(buildUI())

	go func() {
		refreshStatus()
		for range time.Tick(10 * time.Second) {
			refreshStatus()
		}
	}()

	// Check for updates once on startup (non-blocking).
	go checkForUpdateBackground()

	mainWindow.ShowAndRun()
}

// ── UI construction ────────────────────────────────────────────────────────────

func buildUI() fyne.CanvasObject {
	commands := container.NewVScroll(
		container.NewVBox(buildVMSection(), buildBGSection()),
	)
	split := container.NewVSplit(commands, buildOutputArea())
	split.SetOffset(0.62)
	return container.NewBorder(buildStatusBar(), nil, nil, nil, split)
}

func buildStatusBar() fyne.CanvasObject {
	statusDot = canvas.NewCircle(color.NRGBA{R: 120, G: 120, B: 120, A: 255})
	statusDot.StrokeWidth = 0
	dotContainer := container.New(layout.NewGridWrapLayout(fyne.NewSize(14, 14)), statusDot)

	statusLabel = widget.NewLabel("checking…")
	statusLabel.TextStyle = fyne.TextStyle{Bold: true}
	ipLabel = widget.NewLabel("IP: —")

	updateBtn = widget.NewButton("⬆ Update available", cmdApplyUpdate)
	updateBtn.Hide()

	return container.NewHBox(
		dotContainer,
		widget.NewLabel("VM:"),
		statusLabel,
		widget.NewSeparator(),
		ipLabel,
		layout.NewSpacer(),
		updateBtn,
		widget.NewButton("⟳ Refresh", func() { go refreshStatus() }),
		widget.NewButton("↺ Restart Service", cmdRestartService),
		widget.NewButton("🔍 Check updates", func() { go cmdCheckUpdate() }),
		widget.NewButton("⚙ Settings", cmdSettings),
	)
}

func buildVMSection() fyne.CanvasObject {
	btnStartVM = newBtn("Start VM", cmdStartVM)
	btnStopVM = newBtn("Stop VM", cmdStopVM)
	btnRotateSSH = newBtn("Rotate SSH Key", cmdRotateSSH)
	btnChangePass = newBtn("Change Password", cmdChangePassword)

	allButtons = append(allButtons, btnStartVM, btnStopVM, btnRotateSSH, btnChangePass)

	return widget.NewCard("VM Management", "",
		container.NewGridWithColumns(4, btnStartVM, btnStopVM, btnRotateSSH, btnChangePass))
}

func buildBGSection() fyne.CanvasObject {
	btnBGStatus = newBtn("Status", func() { cmdBG("bg-status") })
	btnBGStart = newBtn("Start", func() { cmdBG("bg-start") })
	btnBGRestart = newBtn("Restart", func() { cmdBG("bg-restart") })
	btnBGStop = newBtn("Stop", func() { cmdBG("bg-stop") })
	btnBGUpdate = newBtn("Update", func() { cmdBG("bg-update") })
	btnBGSwap = newBtn("Enable Exp. Swap", func() { cmdBG("bg-swap") })
	btnBGBackup = newBtn("Backup", func() { cmdBG("bg-backup") })
	btnBGBrowser = newBtn("File Browser", cmdFileBrowser)
	btnBGDirector = newBtn("Director", cmdDirector)

	allButtons = append(allButtons,
		btnBGStatus, btnBGStart, btnBGRestart, btnBGStop, btnBGUpdate, btnBGSwap,
		btnBGBackup, btnBGBrowser, btnBGDirector,
	)

	row := func(items ...fyne.CanvasObject) *fyne.Container { return container.NewHBox(items...) }

	return widget.NewCard("Battlegroup Management", "", container.NewVBox(
		row(btnBGStatus, btnBGStart, btnBGRestart, btnBGStop, btnBGUpdate, btnBGSwap),
		widget.NewSeparator(),
		row(widget.NewLabelWithStyle("Database:", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			btnBGBackup),
		row(widget.NewLabelWithStyle("Monitoring:", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			btnBGBrowser, btnBGDirector),
	))
}

func buildOutputArea() fyne.CanvasObject {
	outputEntry = widget.NewMultiLineEntry()
	outputEntry.Wrapping = fyne.TextWrapOff
	outputEntry.SetPlaceHolder("Command output will appear here…")

	outputScroll = container.NewVScroll(outputEntry)
	outputScroll.SetMinSize(fyne.NewSize(900, 200))

	clearBtn := widget.NewButton("Clear", func() {
		outputMu.Lock()
		outputBuf.Reset()
		outputMu.Unlock()
		outputEntry.SetText("")
	})

	killBtn = widget.NewButton("⏹ Kill", func() {
		go func() {
			if err := svcClient.Kill(); err != nil {
				appendOutput(fmt.Sprintf("Kill failed: %v\n", err))
			}
		}()
	})

	header := container.NewHBox(
		widget.NewLabelWithStyle("Output", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		layout.NewSpacer(),
		killBtn,
		clearBtn,
	)
	return container.NewBorder(header, nil, nil, nil, outputScroll)
}

func newBtn(label string, fn func()) *widget.Button {
	return widget.NewButton(label, fn)
}

// ── status refresh ─────────────────────────────────────────────────────────────

func refreshStatus() {
	status, err := svcClient.GetStatus()
	statusMu.Lock()
	if err != nil {
		currentStatus = nil
	} else {
		currentStatus = status
	}
	statusMu.Unlock()
	fyne.Do(func() {
		updateStatusUI(status)
		updateButtonStates(status)
	})
}

func updateStatusUI(status *api.StatusResponse) {
	if status == nil {
		statusDot.FillColor = color.NRGBA{R: 200, G: 40, B: 40, A: 255}
		statusDot.Refresh()
		statusLabel.SetText("Service Offline")
		ipLabel.SetText("IP: —")
		return
	}

	var dotColor color.NRGBA
	var label string
	switch {
	case status.Running:
		dotColor = color.NRGBA{R: 0, G: 200, B: 80, A: 255}
		label = "Running"
	case status.VMState == "Off":
		dotColor = color.NRGBA{R: 200, G: 120, B: 0, A: 255}
		label = "Off"
	case status.VMState == "missing":
		dotColor = color.NRGBA{R: 200, G: 40, B: 40, A: 255}
		label = "Not installed"
	case status.VMState == "Saved":
		dotColor = color.NRGBA{R: 100, G: 100, B: 200, A: 255}
		label = "Saved"
	default:
		dotColor = color.NRGBA{R: 120, G: 120, B: 120, A: 255}
		label = status.VMState
	}
	statusDot.FillColor = dotColor
	statusDot.Refresh()
	statusLabel.SetText(label)
	if status.IP != "" {
		ipLabel.SetText("IP: " + status.IP)
	} else {
		ipLabel.SetText("IP: —")
	}
}

func updateButtonStates(status *api.StatusResponse) {
	cmdMu.Lock()
	running := cmdRunning
	cmdMu.Unlock()
	if running {
		return
	}
	if status == nil {
		for _, btn := range allButtons {
			btn.Disable()
		}
		return
	}

	set := func(btn *widget.Button, enabled bool) {
		if enabled {
			btn.Enable()
		} else {
			btn.Disable()
		}
	}
	vmExists := status.Exists
	vmRunning := status.Running
	busy := status.Busy

	set(btnStartVM, vmExists && !vmRunning && !busy)
	set(btnStopVM, vmRunning && !busy)
	set(btnRotateSSH, vmRunning && !busy)
	set(btnChangePass, vmRunning && !busy)

	for _, btn := range []**widget.Button{
		&btnBGStatus, &btnBGStart, &btnBGRestart, &btnBGStop, &btnBGUpdate,
		&btnBGSwap, &btnBGBackup, &btnBGBrowser, &btnBGDirector,
	} {
		set(*btn, vmRunning && !busy)
	}
}

// ── output helpers ─────────────────────────────────────────────────────────────

func appendOutput(text string) {
	outputMu.Lock()
	outputBuf.WriteString(text)
	full := outputBuf.String()
	outputMu.Unlock()
	fyne.Do(func() {
		outputEntry.SetText(full)
		outputScroll.ScrollToBottom()
	})
}

func appendHeader(title string) {
	appendOutput(fmt.Sprintf("\n══ %s ══\n", title))
}

// ── command gate ───────────────────────────────────────────────────────────────

func getStatus() *api.StatusResponse {
	statusMu.RLock()
	defer statusMu.RUnlock()
	return currentStatus
}

func tryExec(fn func()) {
	cmdMu.Lock()
	if cmdRunning {
		cmdMu.Unlock()
		dialog.ShowInformation("Busy", "Another command is still running. Please wait.", mainWindow)
		return
	}
	cmdRunning = true
	cmdMu.Unlock()

	for _, btn := range allButtons {
		btn.Disable()
	}

	go func() {
		defer func() {
			cmdMu.Lock()
			cmdRunning = false
			cmdMu.Unlock()
			go refreshStatus()
		}()
		fn()
	}()
}

// ── VM commands ────────────────────────────────────────────────────────────────

func cmdStartVM() {
	tryExec(func() {
		appendHeader("Start VM")
		if _, err := svcClient.Exec(api.ExecRequest{Cmd: "vm-start"}, appendOutput); err != nil {
			appendOutput(fmt.Sprintf("Error: %v\n", err))
		}
	})
}

func cmdStopVM() {
	dialog.ShowConfirm("Stop VM",
		"Are you sure you want to stop the VM? The battlegroup will go offline.",
		func(ok bool) {
			if !ok {
				return
			}
			tryExec(func() {
				appendHeader("Stop VM")
				if _, err := svcClient.Exec(api.ExecRequest{Cmd: "vm-stop"}, appendOutput); err != nil {
					appendOutput(fmt.Sprintf("Error: %v\n", err))
				}
			})
		}, mainWindow)
}

func cmdRotateSSH() {
	tryExec(func() {
		appendHeader("Rotate SSH Key")
		if _, err := svcClient.Exec(api.ExecRequest{Cmd: "ssh-rotate"}, appendOutput); err != nil {
			appendOutput(fmt.Sprintf("Error: %v\n", err))
		}
	})
}

func cmdChangePassword() {
	status := getStatus()
	if status == nil || !status.Running {
		dialog.ShowInformation("Unavailable", "VM must be running to change the password.", mainWindow)
		return
	}

	pw1 := widget.NewPasswordEntry()
	pw1.SetPlaceHolder("New password")
	pw2 := widget.NewPasswordEntry()
	pw2.SetPlaceHolder("Confirm password")

	form := &widget.Form{
		Items: []*widget.FormItem{
			{Text: "New password", Widget: pw1},
			{Text: "Confirm", Widget: pw2},
		},
		OnSubmit: func() {
			if pw1.Text == "" {
				dialog.ShowInformation("Error", "Password cannot be empty.", mainWindow)
				return
			}
			if pw1.Text != pw2.Text {
				dialog.ShowInformation("Error", "Passwords do not match.", mainWindow)
				return
			}
			pass := pw1.Text
			tryExec(func() {
				appendHeader("Change Password")
				if _, err := svcClient.Exec(api.ExecRequest{Cmd: "password-change", Password: pass}, appendOutput); err != nil {
					appendOutput(fmt.Sprintf("Error: %v\n", err))
				}
			})
		},
		OnCancel: func() {},
	}
	dialog.ShowCustom("Change Password", "Cancel", form, mainWindow)
}

// ── Battlegroup commands ───────────────────────────────────────────────────────

func cmdBG(cmd string) {
	tryExec(func() {
		appendHeader(cmd)
		if _, err := svcClient.Exec(api.ExecRequest{Cmd: cmd}, appendOutput); err != nil {
			appendOutput(fmt.Sprintf("Error: %v\n", err))
		}
	})
}

func cmdFileBrowser() {
	status := getStatus()
	if status == nil || !status.Running || status.IP == "" {
		return
	}
	url := fmt.Sprintf("http://%s:18888/", status.IP)
	runner.OpenBrowser(url)
	appendOutput(fmt.Sprintf("Opened file browser: %s\n", url))
}

func cmdDirector() {
	status := getStatus()
	if status == nil || !status.Running || status.IP == "" {
		return
	}
	tryExec(func() {
		appendHeader("Open Director")
		url, err := svcClient.Exec(api.ExecRequest{Cmd: "director-port"}, appendOutput)
		if err != nil {
			appendOutput(fmt.Sprintf("Error: %v\n", err))
			return
		}
		if url != "" {
			runner.OpenBrowser(url)
			appendOutput(fmt.Sprintf("Opened Director: %s\n", url))
		}
	})
}

// ── Settings dialog ────────────────────────────────────────────────────────────

func cmdSettings() {
	cfg := config.Get()

	portEntry := widget.NewEntry()
	portEntry.SetText(strconv.Itoa(cfg.Port))
	vmNameEntry := widget.NewEntry()
	vmNameEntry.SetText(cfg.VMName)
	scriptsDirEntry := widget.NewEntry()
	scriptsDirEntry.SetText(cfg.ScriptsDir)
	sshKeyEntry := widget.NewEntry()
	sshKeyEntry.SetText(cfg.SSHKeyPath)

	discordTokenEntry := widget.NewPasswordEntry()
	discordTokenEntry.SetPlaceHolder("(leave empty to disable bot)")
	discordTokenEntry.SetText(cfg.DiscordToken)
	discordGuildEntry := widget.NewEntry()
	discordGuildEntry.SetPlaceHolder("(optional — guild ID for instant command registration)")
	discordGuildEntry.SetText(cfg.DiscordGuildID)
	discordChanEntry := widget.NewEntry()
	discordChanEntry.SetPlaceHolder("(optional — restrict commands to this channel ID)")
	discordChanEntry.SetText(cfg.DiscordChannelID)
	discordRoleSelect := widget.NewSelect(nil, nil)
	discordRoleStatus := widget.NewLabel("")
	discordRoleStatus.Wrapping = fyne.TextWrapWord
	refreshRolesBtn := widget.NewButton("Load Roles", nil)

	githubRepoEntry := widget.NewEntry()
	githubRepoEntry.SetPlaceHolder("owner/repo — e.g. alice/dune-manager")
	githubRepoEntry.SetText(cfg.GitHubRepo)

	const anyRoleLabel = "Any role (no restriction)"
	roleLabelByID := map[string]string{"": anyRoleLabel}
	roleIDByLabel := map[string]string{anyRoleLabel: ""}
	selectedRoleID := cfg.DiscordRoleID

	setRoleOptions := func(roles []discord.Role) {
		labels := []string{anyRoleLabel}
		roleLabelByID = map[string]string{"": anyRoleLabel}
		roleIDByLabel = map[string]string{anyRoleLabel: ""}
		for _, role := range roles {
			label := fmt.Sprintf("%s (%s)", role.Name, role.ID)
			roleLabelByID[role.ID] = label
			roleIDByLabel[label] = role.ID
			labels = append(labels, label)
		}
		if selectedRoleID != "" {
			if _, ok := roleLabelByID[selectedRoleID]; !ok {
				label := fmt.Sprintf("Configured role (%s)", selectedRoleID)
				roleLabelByID[selectedRoleID] = label
				roleIDByLabel[label] = selectedRoleID
				labels = append(labels, label)
			}
		}
		discordRoleSelect.Options = labels
		discordRoleSelect.Refresh()
		discordRoleSelect.SetSelected(roleLabelByID[selectedRoleID])
	}
	setRoleOptions(nil)
	discordRoleSelect.OnChanged = func(label string) {
		selectedRoleID = roleIDByLabel[label]
	}
	discordRoleStatus.SetText("Enter Discord token and guild ID, then load roles to restrict bot access.")

	invalidateRoles := func() {
		selectedRoleID = ""
		setRoleOptions(nil)
		discordRoleStatus.SetText("Discord token or guild changed. Load roles again to choose an allowed role.")
	}
	discordTokenEntry.OnChanged = func(string) { invalidateRoles() }
	discordGuildEntry.OnChanged = func(string) { invalidateRoles() }

	loadRoles := func(showErr bool) {
		token := strings.TrimSpace(discordTokenEntry.Text)
		guildID := strings.TrimSpace(discordGuildEntry.Text)
		if token == "" || guildID == "" {
			invalidateRoles()
			return
		}
		refreshRolesBtn.Disable()
		discordRoleStatus.SetText("Loading Discord roles…")
		go func(token, guildID string) {
			roles, err := discord.ListRoles(token, guildID)
			fyne.Do(func() {
				refreshRolesBtn.Enable()
				if err != nil {
					setRoleOptions(nil)
					discordRoleStatus.SetText("Could not load Discord roles. Check the token and guild ID, or leave bot access unrestricted.")
					if showErr {
						dialog.ShowError(err, mainWindow)
					}
					return
				}
				setRoleOptions(roles)
				if len(roles) == 0 {
					discordRoleStatus.SetText("No selectable roles found. Leave unrestricted or create a Discord role first.")
				} else {
					discordRoleStatus.SetText(fmt.Sprintf("Loaded %d Discord roles.", len(roles)))
				}
			})
		}(token, guildID)
	}
	refreshRolesBtn.OnTapped = func() { loadRoles(true) }
	roleSelector := container.NewVBox(
		container.NewBorder(nil, nil, nil, refreshRolesBtn, discordRoleSelect),
		discordRoleStatus,
	)

	form := &widget.Form{
		Items: []*widget.FormItem{
			{Text: "Service Port", Widget: portEntry},
			{Text: "VM Name", Widget: vmNameEntry},
			{Text: "Scripts Directory", Widget: scriptsDirEntry},
			{Text: "SSH Key Path", Widget: sshKeyEntry},
			{Text: "Discord Token", Widget: discordTokenEntry},
			{Text: "Discord Guild ID", Widget: discordGuildEntry},
			{Text: "Discord Channel ID", Widget: discordChanEntry},
			{Text: "Discord Role", Widget: roleSelector},
			{Text: "GitHub Repo", Widget: githubRepoEntry},
		},
		OnSubmit: func() {
			port, err := strconv.Atoi(portEntry.Text)
			if err != nil || port < 1 || port > 65535 {
				dialog.ShowInformation("Error", "Port must be a number between 1 and 65535.", mainWindow)
				return
			}
			config.Set(config.File{
				Port:             port,
				VMName:           vmNameEntry.Text,
				ScriptsDir:       scriptsDirEntry.Text,
				SSHKeyPath:       sshKeyEntry.Text,
				DiscordToken:     discordTokenEntry.Text,
				DiscordGuildID:   discordGuildEntry.Text,
				DiscordChannelID: discordChanEntry.Text,
				DiscordRoleID:    strings.TrimSpace(selectedRoleID),
				GitHubRepo:       githubRepoEntry.Text,
			})
			if err := config.Save(); err != nil {
				dialog.ShowError(err, mainWindow)
				return
			}
			svcClient = api.NewClient()
			dialog.ShowInformation("Saved",
				"Settings saved.\nRestart the service for Discord / port changes to take effect.", mainWindow)
		},
		OnCancel: func() {},
	}
	scroll := container.NewVScroll(form)
	scroll.SetMinSize(fyne.NewSize(520, 420))
	d := dialog.NewCustom("Settings", "Cancel", scroll, mainWindow)
	d.Resize(fyne.NewSize(580, 500))
	d.Show()
	if strings.TrimSpace(cfg.DiscordToken) != "" && strings.TrimSpace(cfg.DiscordGuildID) != "" {
		loadRoles(false)
	}
}

// ── auto-update ────────────────────────────────────────────────────────────────

// cmdRestartService asks the user to confirm, then restarts the background service.
func cmdRestartService() {
	dialog.ShowConfirm("Restart Service",
		"Restart the Dune Manager background service?\nThis will not affect the VM or battlegroup.",
		func(ok bool) {
			if !ok {
				return
			}
			appendHeader("Restart Service")
			if err := svcClient.RestartService(); err != nil {
				appendOutput(fmt.Sprintf("Error: %v\n", err))
				return
			}
			appendOutput("Service restarting…\n")
			// Wait briefly then re-connect.
			go func() {
				time.Sleep(2 * time.Second)
				svcClient = api.NewClient()
				refreshStatus()
			}()
		}, mainWindow)
}

// checkForUpdateBackground is called once at startup from a goroutine.
func checkForUpdateBackground() {
	info, err := svcClient.CheckUpdate()
	if err != nil || !info.HasUpdate {
		return
	}
	fyne.Do(func() {
		updateGUIURL = info.GUIURL
		updateBtn.SetText(fmt.Sprintf("⬆ Update %s", info.Latest))
		updateBtn.Show()
	})
}

// cmdCheckUpdate is triggered by the manual "Check updates" button.
func cmdCheckUpdate() {
	info, err := svcClient.CheckUpdate()
	if err != nil {
		fyne.Do(func() {
			dialog.ShowError(err, mainWindow)
		})
		return
	}
	if info.Current == "dev" {
		fyne.Do(func() {
			dialog.ShowInformation("Dev build",
				fmt.Sprintf("Running a dev build — update checks are skipped.\nLatest release: %s", info.Latest),
				mainWindow)
		})
		return
	}
	if !info.HasUpdate {
		fyne.Do(func() {
			dialog.ShowInformation("Up to date",
				fmt.Sprintf("You are running the latest version (%s).", info.Current),
				mainWindow)
		})
		return
	}
	fyne.Do(func() {
		updateGUIURL = info.GUIURL
		updateBtn.SetText(fmt.Sprintf("⬆ Update %s", info.Latest))
		updateBtn.Show()
		dialog.ShowInformation("Update available",
			fmt.Sprintf("Version %s is available (current: %s).\nClick the update button in the status bar to apply.", info.Latest, info.Current),
			mainWindow)
	})
}

// cmdApplyUpdate is triggered when the user clicks the update button.
func cmdApplyUpdate() {
	dialog.ShowConfirm("Apply Update",
		fmt.Sprintf("This will update the service binary and restart it.\nThe GUI will also be updated if available.\n\nProceed?"),
		func(ok bool) {
			if !ok {
				return
			}
			tryExec(func() {
				appendHeader("Apply Update")
				guiURL, err := svcClient.ApplyServiceUpdate(appendOutput)
				if err != nil {
					appendOutput(fmt.Sprintf("Error: %v\n", err))
					return
				}
				// Now update the GUI binary if a URL was returned.
				if guiURL == "" {
					appendOutput("Service updated. No GUI update available.\n")
					return
				}
				appendOutput("Downloading GUI update...\n")
				tmpGUI, err := updater.DownloadToTemp(guiURL, func(dl, total int64) {
					if total > 0 {
						appendOutput(fmt.Sprintf("  %.0f%%\n", float64(dl)/float64(total)*100))
					}
				})
				if err != nil {
					appendOutput(fmt.Sprintf("GUI download failed: %v\n", err))
					return
				}
				appendOutput("Applying GUI update...\n")
				guiPath, err := os.Executable()
				if err != nil {
					appendOutput(fmt.Sprintf("Cannot determine executable path: %v\n", err))
					return
				}
				if err := updater.ApplyUpdate(tmpGUI, guiPath); err != nil {
					appendOutput(fmt.Sprintf("GUI apply failed: %v\n", err))
					return
				}
				appendOutput("GUI updated. Restarting...\n")
				// Launch the new binary as a fully detached process so it
				// survives when the current process exits.
				cmd := exec.Command(guiPath)
				cmd.SysProcAttr = &syscall.SysProcAttr{
					CreationFlags: uint32(windows.CREATE_NEW_PROCESS_GROUP | windows.DETACHED_PROCESS),
				}
				if err := cmd.Start(); err != nil {
					appendOutput(fmt.Sprintf("Restart failed: %v\n", err))
					return
				}
				time.Sleep(500 * time.Millisecond)
				fyne.Do(func() {
					fyneApp.Quit()
				})
			})
		}, mainWindow)
}
