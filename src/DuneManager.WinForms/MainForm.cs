using System.Diagnostics;
using DuneManager.Shared;

namespace DuneManager.WinForms;

internal sealed partial class MainForm : Form
{
    private readonly ConfigStore configStore;
    private ApiClient client;
    private StatusResponse? currentStatus;
    private bool commandRunning;
    private readonly List<Button> commandButtons = [];

    public MainForm() : this(new ConfigStore())
    {
    }

    public MainForm(ConfigStore configStore)
    {
        this.configStore = configStore;
        client = new ApiClient(configStore.Snapshot());

        InitializeComponent();
        WireEvents();
        RegisterCommandButtons();

        statusTimer.Start();
        Shown += async (_, _) => await RefreshStatusAsync();
    }

    private void WireEvents()
    {
        statusTimer.Tick += async (_, _) => await RefreshStatusAsync();
        refreshButton.Click += async (_, _) => await RefreshStatusAsync();
        restartButton.Click += (_, _) => StartBackendConsole();
        checkUpdatesButton.Click += async (_, _) => await CheckUpdatesAsync();
        settingsButton.Click += (_, _) => ShowSettings();
        clearButton.Click += (_, _) => outputBox.Clear();
        killButton.Click += async (_, _) => await client.KillAsync();

        btnStartVm.Click += async (_, _) => await ExecAsync("Start VM", "vm-start");
        btnStopVm.Click += async (_, _) => await ConfirmStopVmAsync();
        btnRotateSsh.Click += async (_, _) => await RotateSshAsync();
        btnChangePassword.Click += async (_, _) => await ChangePasswordAsync();

        btnBgStatus.Click += async (_, _) => await ExecAsync("Battlegroup Status", "bg-status");
        btnBgStart.Click += async (_, _) => await ExecAsync("Battlegroup Start", "bg-start");
        btnBgRestart.Click += async (_, _) => await ExecAsync("Battlegroup Restart", "bg-restart");
        btnBgStop.Click += async (_, _) => await ExecAsync("Battlegroup Stop", "bg-stop");
        btnBgUpdate.Click += async (_, _) => await ExecAsync("Battlegroup Update", "bg-update");
        btnBgSwap.Click += async (_, _) => await ExecAsync("Enable Experimental Swap", "bg-swap");
        btnBgBackup.Click += async (_, _) => await ExecAsync("Battlegroup Backup", "bg-backup");
        btnFileBrowser.Click += async (_, _) => await OpenFileBrowser();
        btnDirector.Click += async (_, _) => await OpenDirectorAsync();
    }

    private void RegisterCommandButtons()
    {
        commandButtons.AddRange([
            btnStartVm,
            btnStopVm,
            btnRotateSsh,
            btnChangePassword,
            btnBgStatus,
            btnBgStart,
            btnBgRestart,
            btnBgStop,
            btnBgUpdate,
            btnBgSwap,
            btnBgBackup,
            btnFileBrowser,
            btnDirector
        ]);
    }

    private async Task RefreshStatusAsync()
    {
        currentStatus = await client.GetStatusAsync();
        if (currentStatus is null)
        {
            statusDot.ForeColor = Color.Firebrick;
            statusLabel.Text = "Backend Offline";
            ipLabel.Text = "IP: —";
            restartButton.Text = "▶ Start Backend";
        }
        else
        {
            restartButton.Text = "Backend Online";
            statusDot.ForeColor = currentStatus.Running ? Color.SeaGreen : currentStatus.VMState == "missing" ? Color.Firebrick : Color.DarkOrange;
            statusLabel.Text = currentStatus.Running ? "Running" : currentStatus.VMState;
            ipLabel.Text = string.IsNullOrWhiteSpace(currentStatus.IP) ? "IP: —" : "IP: " + currentStatus.IP;
        }
        UpdateButtonStates();
    }

    private void UpdateButtonStates()
    {
        var enableVmActions = !commandRunning && currentStatus is { Running: true, Busy: false };
        foreach (var button in commandButtons)
        {
            button.Enabled = enableVmActions;
        }

        btnStartVm.Enabled = !commandRunning && currentStatus is { Exists: true, Running: false, Busy: false };
    }

    private async Task<string> ExecAsync(string title, string command, string password = "")
    {
        if (commandRunning)
        {
            MessageBox.Show(this, "Another command is still running.", "Busy", MessageBoxButtons.OK, MessageBoxIcon.Information);
            return string.Empty;
        }
        commandRunning = true;
        UpdateButtonStates();
        AppendHeader(title);
        try
        {
            return await client.ExecAsync(new ExecRequest { Cmd = command, Password = password }, AppendOutput);
        }
        catch (Exception ex)
        {
            AppendOutput("Error: " + ex.Message + Environment.NewLine);
        }
        finally
        {
            commandRunning = false;
            await RefreshStatusAsync();
        }
        return string.Empty;
    }

    private async Task ConfirmStopVmAsync()
    {
        if (MessageBox.Show(this, "Are you sure you want to stop the VM? The battlegroup will go offline.", "Stop VM", MessageBoxButtons.YesNo, MessageBoxIcon.Warning) == DialogResult.Yes)
        {
            await ExecAsync("Stop VM", "vm-stop");
        }
    }

    private async Task RotateSshAsync()
    {
        var password = PromptPassword("Rotate SSH Key", "Current 'dune' user password (optional recovery fallback):");
        if (password is not null)
        {
            await ExecAsync("Rotate SSH Key", "ssh-rotate", password);
        }
    }

    private async Task ChangePasswordAsync()
    {
        var password = PromptPassword("Change Password", "New 'dune' user password:");
        if (!string.IsNullOrWhiteSpace(password))
        {
            await ExecAsync("Change Password", "password-change", password);
        }
    }

    private Task OpenFileBrowser()
    {
        if (currentStatus?.IP is { Length: > 0 } ip)
        {
            OpenBrowser($"http://{ip}:18888/");
        }
        return Task.CompletedTask;
    }

    private async Task OpenDirectorAsync()
    {
        var url = await ExecAsync("Open Director", "director-port");
        if (!string.IsNullOrWhiteSpace(url))
        {
            OpenBrowser(url);
            AppendOutput("Opened Director: " + url + Environment.NewLine);
        }
    }

    private void ShowSettings()
    {
        var existing = configStore.Snapshot();
        using var form = new SettingsForm(existing);
        if (form.ShowDialog(this) == DialogResult.OK)
        {
            configStore.Save(form.ToConfig(existing));
            client = new ApiClient(configStore.Snapshot());
            MessageBox.Show(this, "Settings saved. Restart the backend for port changes to take effect.", "Saved", MessageBoxButtons.OK, MessageBoxIcon.Information);
        }
    }

    private async Task CheckUpdatesAsync()
    {
        try
        {
            var update = await client.CheckUpdateAsync();
            MessageBox.Show(this, string.IsNullOrWhiteSpace(update.Error) ? "No update available." : update.Error, "Update check", MessageBoxButtons.OK, MessageBoxIcon.Information);
        }
        catch (Exception ex)
        {
            MessageBox.Show(this, ex.Message, "Update check failed", MessageBoxButtons.OK, MessageBoxIcon.Error);
        }
    }

    private void StartBackendConsole()
    {
        var backendPath = Path.Combine(AppContext.BaseDirectory, "dune-manager-backend.exe");
        if (!File.Exists(backendPath))
        {
            MessageBox.Show(this, "Place dune-manager-backend.exe next to dune-manager.exe, then start it again.", "Backend not found", MessageBoxButtons.OK, MessageBoxIcon.Warning);
            return;
        }

        Process.Start(new ProcessStartInfo(backendPath) { UseShellExecute = true, Verb = "runas" });
    }

    private void AppendHeader(string title) => AppendOutput(Environment.NewLine + "══ " + title + " ══" + Environment.NewLine);

    private void AppendOutput(string text)
    {
        if (InvokeRequired)
        {
            BeginInvoke(new Action(() => AppendOutput(text)));
            return;
        }
        outputBox.AppendText(text);
    }

    private static void OpenBrowser(string url)
    {
        Process.Start(new ProcessStartInfo(url) { UseShellExecute = true });
    }

    private static string? PromptPassword(string title, string label)
    {
        using var form = new Form { Text = title, Width = 420, Height = 150, StartPosition = FormStartPosition.CenterParent, FormBorderStyle = FormBorderStyle.FixedDialog };
        var textBox = new TextBox { UseSystemPasswordChar = true, Width = 360, Left = 16, Top = 42 };
        form.Controls.Add(new Label { Text = label, AutoSize = true, Left = 16, Top = 16 });
        form.Controls.Add(textBox);
        var ok = new Button { Text = "OK", DialogResult = DialogResult.OK, Left = 210, Width = 80, Top = 76 };
        var cancel = new Button { Text = "Cancel", DialogResult = DialogResult.Cancel, Left = 298, Width = 80, Top = 76 };
        form.Controls.Add(ok);
        form.Controls.Add(cancel);
        form.AcceptButton = ok;
        form.CancelButton = cancel;
        return form.ShowDialog() == DialogResult.OK ? textBox.Text : null;
    }
}
