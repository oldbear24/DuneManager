namespace DuneManager.WinForms;

partial class MainForm
{
    private System.ComponentModel.IContainer components = null!;
    private System.Windows.Forms.Timer statusTimer = null!;
    private TableLayoutPanel rootLayout = null!;
    private FlowLayoutPanel statusBarPanel = null!;
    private Label statusDot = null!;
    private Label vmCaptionLabel = null!;
    private Label statusLabel = null!;
    private Label statusSeparatorLabel = null!;
    private Label ipLabel = null!;
    private Button refreshButton = null!;
    private Button restartButton = null!;
    private Button checkUpdatesButton = null!;
    private Button settingsButton = null!;
    private TabControl commandTabs = null!;
    private TabPage vmTabPage = null!;
    private TabPage battlegroupTabPage = null!;
    private FlowLayoutPanel vmCommandPanel = null!;
    private Button btnStartVm = null!;
    private Button btnStopVm = null!;
    private Button btnRotateSsh = null!;
    private Button btnChangePassword = null!;
    private FlowLayoutPanel battlegroupCommandPanel = null!;
    private Button btnBgStatus = null!;
    private Button btnBgStart = null!;
    private Button btnBgRestart = null!;
    private Button btnBgStop = null!;
    private Button btnBgUpdate = null!;
    private Button btnBgSwap = null!;
    private Button btnBgBackup = null!;
    private Button btnFileBrowser = null!;
    private Button btnDirector = null!;
    private Panel outputPanel = null!;
    private FlowLayoutPanel outputHeaderPanel = null!;
    private Label outputLabel = null!;
    private Button killButton = null!;
    private Button clearButton = null!;
    private TextBox outputBox = null!;

    protected override void Dispose(bool disposing)
    {
        if (disposing && (components is not null))
        {
            components.Dispose();
        }
        base.Dispose(disposing);
    }

    private void InitializeComponent()
    {
        components = new System.ComponentModel.Container();
        statusTimer = new System.Windows.Forms.Timer(components);
        rootLayout = new TableLayoutPanel();
        statusBarPanel = new FlowLayoutPanel();
        statusDot = new Label();
        vmCaptionLabel = new Label();
        statusLabel = new Label();
        statusSeparatorLabel = new Label();
        ipLabel = new Label();
        refreshButton = new Button();
        restartButton = new Button();
        checkUpdatesButton = new Button();
        settingsButton = new Button();
        commandTabs = new TabControl();
        vmTabPage = new TabPage();
        vmCommandPanel = new FlowLayoutPanel();
        btnStartVm = new Button();
        btnStopVm = new Button();
        btnRotateSsh = new Button();
        btnChangePassword = new Button();
        battlegroupTabPage = new TabPage();
        battlegroupCommandPanel = new FlowLayoutPanel();
        btnBgStatus = new Button();
        btnBgStart = new Button();
        btnBgRestart = new Button();
        btnBgStop = new Button();
        btnBgUpdate = new Button();
        btnBgSwap = new Button();
        btnBgBackup = new Button();
        btnFileBrowser = new Button();
        btnDirector = new Button();
        outputPanel = new Panel();
        outputBox = new TextBox();
        outputHeaderPanel = new FlowLayoutPanel();
        outputLabel = new Label();
        killButton = new Button();
        clearButton = new Button();
        rootLayout.SuspendLayout();
        statusBarPanel.SuspendLayout();
        commandTabs.SuspendLayout();
        vmTabPage.SuspendLayout();
        vmCommandPanel.SuspendLayout();
        battlegroupTabPage.SuspendLayout();
        battlegroupCommandPanel.SuspendLayout();
        outputPanel.SuspendLayout();
        outputHeaderPanel.SuspendLayout();
        SuspendLayout();
        //
        // statusTimer
        //
        statusTimer.Interval = 10000;
        //
        // rootLayout
        //
        rootLayout.ColumnCount = 1;
        rootLayout.ColumnStyles.Add(new ColumnStyle(SizeType.Percent, 100F));
        rootLayout.Controls.Add(statusBarPanel, 0, 0);
        rootLayout.Controls.Add(commandTabs, 0, 1);
        rootLayout.Controls.Add(outputPanel, 0, 2);
        rootLayout.Dock = DockStyle.Fill;
        rootLayout.Location = new Point(0, 0);
        rootLayout.Name = "rootLayout";
        rootLayout.RowCount = 3;
        rootLayout.RowStyles.Add(new RowStyle(SizeType.AutoSize));
        rootLayout.RowStyles.Add(new RowStyle(SizeType.Percent, 58F));
        rootLayout.RowStyles.Add(new RowStyle(SizeType.Percent, 42F));
        rootLayout.Size = new Size(964, 701);
        rootLayout.TabIndex = 0;
        //
        // statusBarPanel
        //
        statusBarPanel.AutoSize = true;
        statusBarPanel.Controls.Add(statusDot);
        statusBarPanel.Controls.Add(vmCaptionLabel);
        statusBarPanel.Controls.Add(statusLabel);
        statusBarPanel.Controls.Add(statusSeparatorLabel);
        statusBarPanel.Controls.Add(ipLabel);
        statusBarPanel.Controls.Add(refreshButton);
        statusBarPanel.Controls.Add(restartButton);
        statusBarPanel.Controls.Add(checkUpdatesButton);
        statusBarPanel.Controls.Add(settingsButton);
        statusBarPanel.Dock = DockStyle.Top;
        statusBarPanel.Location = new Point(0, 0);
        statusBarPanel.Margin = new Padding(0);
        statusBarPanel.Name = "statusBarPanel";
        statusBarPanel.Padding = new Padding(8);
        statusBarPanel.Size = new Size(964, 47);
        statusBarPanel.TabIndex = 0;
        statusBarPanel.WrapContents = false;
        //
        // statusDot
        //
        statusDot.AutoSize = true;
        statusDot.Font = new Font("Segoe UI", 18F, FontStyle.Bold);
        statusDot.ForeColor = Color.Gray;
        statusDot.Location = new Point(11, 8);
        statusDot.Name = "statusDot";
        statusDot.Size = new Size(29, 32);
        statusDot.TabIndex = 0;
        statusDot.Text = "●";
        //
        // vmCaptionLabel
        //
        vmCaptionLabel.AutoSize = true;
        vmCaptionLabel.Location = new Point(43, 16);
        vmCaptionLabel.Margin = new Padding(0, 8, 0, 0);
        vmCaptionLabel.Name = "vmCaptionLabel";
        vmCaptionLabel.Size = new Size(27, 15);
        vmCaptionLabel.TabIndex = 1;
        vmCaptionLabel.Text = "VM:";
        //
        // statusLabel
        //
        statusLabel.AutoSize = true;
        statusLabel.Font = new Font("Segoe UI", 9F, FontStyle.Bold);
        statusLabel.Location = new Point(73, 16);
        statusLabel.Margin = new Padding(3, 8, 3, 0);
        statusLabel.Name = "statusLabel";
        statusLabel.Size = new Size(67, 15);
        statusLabel.TabIndex = 2;
        statusLabel.Text = "checking…";
        //
        // statusSeparatorLabel
        //
        statusSeparatorLabel.AutoSize = true;
        statusSeparatorLabel.Location = new Point(146, 16);
        statusSeparatorLabel.Margin = new Padding(3, 8, 3, 0);
        statusSeparatorLabel.Name = "statusSeparatorLabel";
        statusSeparatorLabel.Size = new Size(10, 15);
        statusSeparatorLabel.TabIndex = 3;
        statusSeparatorLabel.Text = "|";
        //
        // ipLabel
        //
        ipLabel.AutoSize = true;
        ipLabel.Location = new Point(162, 16);
        ipLabel.Margin = new Padding(3, 8, 12, 0);
        ipLabel.Name = "ipLabel";
        ipLabel.Size = new Size(35, 15);
        ipLabel.TabIndex = 4;
        ipLabel.Text = "IP: —";
        //
        // refreshButton
        //
        refreshButton.AutoSize = true;
        refreshButton.Location = new Point(212, 12);
        refreshButton.Margin = new Padding(3, 4, 3, 3);
        refreshButton.Name = "refreshButton";
        refreshButton.Size = new Size(79, 25);
        refreshButton.TabIndex = 5;
        refreshButton.Text = "⟳ Refresh";
        refreshButton.UseVisualStyleBackColor = true;
        //
        // restartButton
        //
        restartButton.AutoSize = true;
        restartButton.Location = new Point(297, 12);
        restartButton.Margin = new Padding(3, 4, 3, 3);
        restartButton.Name = "restartButton";
        restartButton.Size = new Size(114, 25);
        restartButton.TabIndex = 6;
        restartButton.Text = "▶ Start Backend";
        restartButton.UseVisualStyleBackColor = true;
        //
        // checkUpdatesButton
        //
        checkUpdatesButton.AutoSize = true;
        checkUpdatesButton.Location = new Point(417, 12);
        checkUpdatesButton.Margin = new Padding(3, 4, 3, 3);
        checkUpdatesButton.Name = "checkUpdatesButton";
        checkUpdatesButton.Size = new Size(117, 25);
        checkUpdatesButton.TabIndex = 7;
        checkUpdatesButton.Text = "🔍 Check updates";
        checkUpdatesButton.UseVisualStyleBackColor = true;
        //
        // settingsButton
        //
        settingsButton.AutoSize = true;
        settingsButton.Location = new Point(540, 12);
        settingsButton.Margin = new Padding(3, 4, 3, 3);
        settingsButton.Name = "settingsButton";
        settingsButton.Size = new Size(85, 25);
        settingsButton.TabIndex = 8;
        settingsButton.Text = "⚙ Settings";
        settingsButton.UseVisualStyleBackColor = true;
        //
        // commandTabs
        //
        commandTabs.Controls.Add(vmTabPage);
        commandTabs.Controls.Add(battlegroupTabPage);
        commandTabs.Dock = DockStyle.Fill;
        commandTabs.Location = new Point(3, 50);
        commandTabs.Name = "commandTabs";
        commandTabs.SelectedIndex = 0;
        commandTabs.Size = new Size(958, 373);
        commandTabs.TabIndex = 1;
        //
        // vmTabPage
        //
        vmTabPage.Controls.Add(vmCommandPanel);
        vmTabPage.Location = new Point(4, 24);
        vmTabPage.Name = "vmTabPage";
        vmTabPage.Padding = new Padding(12);
        vmTabPage.Size = new Size(950, 345);
        vmTabPage.TabIndex = 0;
        vmTabPage.Text = "VM Management";
        vmTabPage.UseVisualStyleBackColor = true;
        //
        // vmCommandPanel
        //
        vmCommandPanel.Controls.Add(btnStartVm);
        vmCommandPanel.Controls.Add(btnStopVm);
        vmCommandPanel.Controls.Add(btnRotateSsh);
        vmCommandPanel.Controls.Add(btnChangePassword);
        vmCommandPanel.Dock = DockStyle.Fill;
        vmCommandPanel.Location = new Point(12, 12);
        vmCommandPanel.Name = "vmCommandPanel";
        vmCommandPanel.Size = new Size(926, 321);
        vmCommandPanel.TabIndex = 0;
        //
        // btnStartVm
        //
        btnStartVm.Location = new Point(8, 8);
        btnStartVm.Margin = new Padding(8);
        btnStartVm.Name = "btnStartVm";
        btnStartVm.Size = new Size(150, 42);
        btnStartVm.TabIndex = 0;
        btnStartVm.Text = "Start VM";
        btnStartVm.UseVisualStyleBackColor = true;
        //
        // btnStopVm
        //
        btnStopVm.Location = new Point(174, 8);
        btnStopVm.Margin = new Padding(8);
        btnStopVm.Name = "btnStopVm";
        btnStopVm.Size = new Size(150, 42);
        btnStopVm.TabIndex = 1;
        btnStopVm.Text = "Stop VM";
        btnStopVm.UseVisualStyleBackColor = true;
        //
        // btnRotateSsh
        //
        btnRotateSsh.Location = new Point(340, 8);
        btnRotateSsh.Margin = new Padding(8);
        btnRotateSsh.Name = "btnRotateSsh";
        btnRotateSsh.Size = new Size(150, 42);
        btnRotateSsh.TabIndex = 2;
        btnRotateSsh.Text = "Rotate SSH Key";
        btnRotateSsh.UseVisualStyleBackColor = true;
        //
        // btnChangePassword
        //
        btnChangePassword.Location = new Point(506, 8);
        btnChangePassword.Margin = new Padding(8);
        btnChangePassword.Name = "btnChangePassword";
        btnChangePassword.Size = new Size(150, 42);
        btnChangePassword.TabIndex = 3;
        btnChangePassword.Text = "Change Password";
        btnChangePassword.UseVisualStyleBackColor = true;
        //
        // battlegroupTabPage
        //
        battlegroupTabPage.Controls.Add(battlegroupCommandPanel);
        battlegroupTabPage.Location = new Point(4, 24);
        battlegroupTabPage.Name = "battlegroupTabPage";
        battlegroupTabPage.Padding = new Padding(12);
        battlegroupTabPage.Size = new Size(950, 345);
        battlegroupTabPage.TabIndex = 1;
        battlegroupTabPage.Text = "Battlegroup Management";
        battlegroupTabPage.UseVisualStyleBackColor = true;
        //
        // battlegroupCommandPanel
        //
        battlegroupCommandPanel.Controls.Add(btnBgStatus);
        battlegroupCommandPanel.Controls.Add(btnBgStart);
        battlegroupCommandPanel.Controls.Add(btnBgRestart);
        battlegroupCommandPanel.Controls.Add(btnBgStop);
        battlegroupCommandPanel.Controls.Add(btnBgUpdate);
        battlegroupCommandPanel.Controls.Add(btnBgSwap);
        battlegroupCommandPanel.Controls.Add(btnBgBackup);
        battlegroupCommandPanel.Controls.Add(btnFileBrowser);
        battlegroupCommandPanel.Controls.Add(btnDirector);
        battlegroupCommandPanel.Dock = DockStyle.Fill;
        battlegroupCommandPanel.Location = new Point(12, 12);
        battlegroupCommandPanel.Name = "battlegroupCommandPanel";
        battlegroupCommandPanel.Size = new Size(926, 321);
        battlegroupCommandPanel.TabIndex = 0;
        //
        // btnBgStatus
        //
        btnBgStatus.Location = new Point(8, 8);
        btnBgStatus.Margin = new Padding(8);
        btnBgStatus.Name = "btnBgStatus";
        btnBgStatus.Size = new Size(150, 42);
        btnBgStatus.TabIndex = 0;
        btnBgStatus.Text = "Status";
        btnBgStatus.UseVisualStyleBackColor = true;
        //
        // btnBgStart
        //
        btnBgStart.Location = new Point(174, 8);
        btnBgStart.Margin = new Padding(8);
        btnBgStart.Name = "btnBgStart";
        btnBgStart.Size = new Size(150, 42);
        btnBgStart.TabIndex = 1;
        btnBgStart.Text = "Start";
        btnBgStart.UseVisualStyleBackColor = true;
        //
        // btnBgRestart
        //
        btnBgRestart.Location = new Point(340, 8);
        btnBgRestart.Margin = new Padding(8);
        btnBgRestart.Name = "btnBgRestart";
        btnBgRestart.Size = new Size(150, 42);
        btnBgRestart.TabIndex = 2;
        btnBgRestart.Text = "Restart";
        btnBgRestart.UseVisualStyleBackColor = true;
        //
        // btnBgStop
        //
        btnBgStop.Location = new Point(506, 8);
        btnBgStop.Margin = new Padding(8);
        btnBgStop.Name = "btnBgStop";
        btnBgStop.Size = new Size(150, 42);
        btnBgStop.TabIndex = 3;
        btnBgStop.Text = "Stop";
        btnBgStop.UseVisualStyleBackColor = true;
        //
        // btnBgUpdate
        //
        btnBgUpdate.Location = new Point(672, 8);
        btnBgUpdate.Margin = new Padding(8);
        btnBgUpdate.Name = "btnBgUpdate";
        btnBgUpdate.Size = new Size(150, 42);
        btnBgUpdate.TabIndex = 4;
        btnBgUpdate.Text = "Update";
        btnBgUpdate.UseVisualStyleBackColor = true;
        //
        // btnBgSwap
        //
        btnBgSwap.Location = new Point(8, 66);
        btnBgSwap.Margin = new Padding(8);
        btnBgSwap.Name = "btnBgSwap";
        btnBgSwap.Size = new Size(150, 42);
        btnBgSwap.TabIndex = 5;
        btnBgSwap.Text = "Enable Exp. Swap";
        btnBgSwap.UseVisualStyleBackColor = true;
        //
        // btnBgBackup
        //
        btnBgBackup.Location = new Point(174, 66);
        btnBgBackup.Margin = new Padding(8);
        btnBgBackup.Name = "btnBgBackup";
        btnBgBackup.Size = new Size(150, 42);
        btnBgBackup.TabIndex = 6;
        btnBgBackup.Text = "Backup";
        btnBgBackup.UseVisualStyleBackColor = true;
        //
        // btnFileBrowser
        //
        btnFileBrowser.Location = new Point(340, 66);
        btnFileBrowser.Margin = new Padding(8);
        btnFileBrowser.Name = "btnFileBrowser";
        btnFileBrowser.Size = new Size(150, 42);
        btnFileBrowser.TabIndex = 7;
        btnFileBrowser.Text = "File Browser";
        btnFileBrowser.UseVisualStyleBackColor = true;
        //
        // btnDirector
        //
        btnDirector.Location = new Point(506, 66);
        btnDirector.Margin = new Padding(8);
        btnDirector.Name = "btnDirector";
        btnDirector.Size = new Size(150, 42);
        btnDirector.TabIndex = 8;
        btnDirector.Text = "Director";
        btnDirector.UseVisualStyleBackColor = true;
        //
        // outputPanel
        //
        outputPanel.Controls.Add(outputBox);
        outputPanel.Controls.Add(outputHeaderPanel);
        outputPanel.Dock = DockStyle.Fill;
        outputPanel.Location = new Point(3, 429);
        outputPanel.Name = "outputPanel";
        outputPanel.Padding = new Padding(8);
        outputPanel.Size = new Size(958, 269);
        outputPanel.TabIndex = 2;
        //
        // outputBox
        //
        outputBox.Dock = DockStyle.Fill;
        outputBox.Font = new Font("Consolas", 10F);
        outputBox.Location = new Point(8, 44);
        outputBox.Multiline = true;
        outputBox.Name = "outputBox";
        outputBox.ReadOnly = true;
        outputBox.ScrollBars = ScrollBars.Both;
        outputBox.Size = new Size(942, 217);
        outputBox.TabIndex = 1;
        outputBox.WordWrap = false;
        //
        // outputHeaderPanel
        //
        outputHeaderPanel.Controls.Add(clearButton);
        outputHeaderPanel.Controls.Add(killButton);
        outputHeaderPanel.Controls.Add(outputLabel);
        outputHeaderPanel.Dock = DockStyle.Top;
        outputHeaderPanel.FlowDirection = FlowDirection.RightToLeft;
        outputHeaderPanel.Location = new Point(8, 8);
        outputHeaderPanel.Name = "outputHeaderPanel";
        outputHeaderPanel.Size = new Size(942, 36);
        outputHeaderPanel.TabIndex = 0;
        //
        // outputLabel
        //
        outputLabel.AutoSize = true;
        outputLabel.Font = new Font("Segoe UI", 9F, FontStyle.Bold);
        outputLabel.Location = new Point(755, 8);
        outputLabel.Margin = new Padding(3, 8, 500, 0);
        outputLabel.Name = "outputLabel";
        outputLabel.Size = new Size(45, 15);
        outputLabel.TabIndex = 2;
        outputLabel.Text = "Output";
        //
        // killButton
        //
        killButton.AutoSize = true;
        killButton.Location = new Point(806, 3);
        killButton.Name = "killButton";
        killButton.Size = new Size(58, 25);
        killButton.TabIndex = 1;
        killButton.Text = "⏹ Kill";
        killButton.UseVisualStyleBackColor = true;
        //
        // clearButton
        //
        clearButton.AutoSize = true;
        clearButton.Location = new Point(870, 3);
        clearButton.Name = "clearButton";
        clearButton.Size = new Size(69, 25);
        clearButton.TabIndex = 0;
        clearButton.Text = "Clear";
        clearButton.UseVisualStyleBackColor = true;
        //
        // MainForm
        //
        AutoScaleDimensions = new SizeF(7F, 15F);
        AutoScaleMode = AutoScaleMode.Font;
        ClientSize = new Size(964, 701);
        Controls.Add(rootLayout);
        Name = "MainForm";
        StartPosition = FormStartPosition.CenterScreen;
        Text = "Dune Awakening — Server Manager";
        rootLayout.ResumeLayout(false);
        rootLayout.PerformLayout();
        statusBarPanel.ResumeLayout(false);
        statusBarPanel.PerformLayout();
        commandTabs.ResumeLayout(false);
        vmTabPage.ResumeLayout(false);
        vmCommandPanel.ResumeLayout(false);
        battlegroupTabPage.ResumeLayout(false);
        battlegroupCommandPanel.ResumeLayout(false);
        outputPanel.ResumeLayout(false);
        outputPanel.PerformLayout();
        outputHeaderPanel.ResumeLayout(false);
        outputHeaderPanel.PerformLayout();
        ResumeLayout(false);
    }
}
