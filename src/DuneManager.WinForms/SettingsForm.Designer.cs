namespace DuneManager.WinForms;

partial class SettingsForm
{
    private System.ComponentModel.IContainer components = null!;
    private TableLayoutPanel settingsTable = null!;
    private Label servicePortLabel = null!;
    private Label vmNameLabel = null!;
    private Label scriptsDirectoryLabel = null!;
    private Label sshKeyPathLabel = null!;
    private Label discordTokenLabel = null!;
    private Label discordGuildLabel = null!;
    private Label discordChannelLabel = null!;
    private Label discordRoleLabel = null!;
    private Label statusChannelLabel = null!;
    private Label githubRepoLabel = null!;
    private NumericUpDown portBox = null!;
    private TextBox vmNameBox = null!;
    private TextBox scriptsDirBox = null!;
    private TextBox sshKeyBox = null!;
    private TextBox discordTokenBox = null!;
    private TextBox discordGuildBox = null!;
    private TextBox discordChannelBox = null!;
    private TextBox discordRoleBox = null!;
    private TextBox discordStatusChannelBox = null!;
    private TextBox githubRepoBox = null!;
    private FlowLayoutPanel buttonPanel = null!;
    private Button saveButton = null!;
    private Button cancelButton = null!;

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
        settingsTable = new TableLayoutPanel();
        servicePortLabel = new Label();
        vmNameLabel = new Label();
        scriptsDirectoryLabel = new Label();
        sshKeyPathLabel = new Label();
        discordTokenLabel = new Label();
        discordGuildLabel = new Label();
        discordChannelLabel = new Label();
        discordRoleLabel = new Label();
        statusChannelLabel = new Label();
        githubRepoLabel = new Label();
        portBox = new NumericUpDown();
        vmNameBox = new TextBox();
        scriptsDirBox = new TextBox();
        sshKeyBox = new TextBox();
        discordTokenBox = new TextBox();
        discordGuildBox = new TextBox();
        discordChannelBox = new TextBox();
        discordRoleBox = new TextBox();
        discordStatusChannelBox = new TextBox();
        githubRepoBox = new TextBox();
        buttonPanel = new FlowLayoutPanel();
        saveButton = new Button();
        cancelButton = new Button();
        settingsTable.SuspendLayout();
        ((System.ComponentModel.ISupportInitialize)portBox).BeginInit();
        buttonPanel.SuspendLayout();
        SuspendLayout();
        //
        // settingsTable
        //
        settingsTable.AutoScroll = true;
        settingsTable.ColumnCount = 2;
        settingsTable.ColumnStyles.Add(new ColumnStyle(SizeType.Absolute, 165F));
        settingsTable.ColumnStyles.Add(new ColumnStyle(SizeType.Percent, 100F));
        settingsTable.Dock = DockStyle.Fill;
        settingsTable.Location = new Point(0, 0);
        settingsTable.Name = "settingsTable";
        settingsTable.Padding = new Padding(12);
        settingsTable.RowCount = 10;
        settingsTable.RowStyles.Add(new RowStyle(SizeType.AutoSize));
        settingsTable.RowStyles.Add(new RowStyle(SizeType.AutoSize));
        settingsTable.RowStyles.Add(new RowStyle(SizeType.AutoSize));
        settingsTable.RowStyles.Add(new RowStyle(SizeType.AutoSize));
        settingsTable.RowStyles.Add(new RowStyle(SizeType.AutoSize));
        settingsTable.RowStyles.Add(new RowStyle(SizeType.AutoSize));
        settingsTable.RowStyles.Add(new RowStyle(SizeType.AutoSize));
        settingsTable.RowStyles.Add(new RowStyle(SizeType.AutoSize));
        settingsTable.RowStyles.Add(new RowStyle(SizeType.AutoSize));
        settingsTable.RowStyles.Add(new RowStyle(SizeType.AutoSize));
        settingsTable.Size = new Size(604, 437);
        settingsTable.TabIndex = 0;
        settingsTable.Controls.Add(servicePortLabel, 0, 0);
        settingsTable.Controls.Add(portBox, 1, 0);
        settingsTable.Controls.Add(vmNameLabel, 0, 1);
        settingsTable.Controls.Add(vmNameBox, 1, 1);
        settingsTable.Controls.Add(scriptsDirectoryLabel, 0, 2);
        settingsTable.Controls.Add(scriptsDirBox, 1, 2);
        settingsTable.Controls.Add(sshKeyPathLabel, 0, 3);
        settingsTable.Controls.Add(sshKeyBox, 1, 3);
        settingsTable.Controls.Add(discordTokenLabel, 0, 4);
        settingsTable.Controls.Add(discordTokenBox, 1, 4);
        settingsTable.Controls.Add(discordGuildLabel, 0, 5);
        settingsTable.Controls.Add(discordGuildBox, 1, 5);
        settingsTable.Controls.Add(discordChannelLabel, 0, 6);
        settingsTable.Controls.Add(discordChannelBox, 1, 6);
        settingsTable.Controls.Add(discordRoleLabel, 0, 7);
        settingsTable.Controls.Add(discordRoleBox, 1, 7);
        settingsTable.Controls.Add(statusChannelLabel, 0, 8);
        settingsTable.Controls.Add(discordStatusChannelBox, 1, 8);
        settingsTable.Controls.Add(githubRepoLabel, 0, 9);
        settingsTable.Controls.Add(githubRepoBox, 1, 9);
        //
        // servicePortLabel
        //
        servicePortLabel.Anchor = AnchorStyles.Left;
        servicePortLabel.AutoSize = true;
        servicePortLabel.Location = new Point(15, 18);
        servicePortLabel.Name = "servicePortLabel";
        servicePortLabel.Padding = new Padding(0, 6, 0, 0);
        servicePortLabel.Size = new Size(100, 21);
        servicePortLabel.TabIndex = 0;
        servicePortLabel.Text = "Service Port";
        //
        // vmNameLabel
        //
        vmNameLabel.Anchor = AnchorStyles.Left;
        vmNameLabel.AutoSize = true;
        vmNameLabel.Location = new Point(15, 47);
        vmNameLabel.Name = "vmNameLabel";
        vmNameLabel.Padding = new Padding(0, 6, 0, 0);
        vmNameLabel.Size = new Size(100, 21);
        vmNameLabel.TabIndex = 2;
        vmNameLabel.Text = "VM Name";
        //
        // scriptsDirectoryLabel
        //
        scriptsDirectoryLabel.Anchor = AnchorStyles.Left;
        scriptsDirectoryLabel.AutoSize = true;
        scriptsDirectoryLabel.Location = new Point(15, 76);
        scriptsDirectoryLabel.Name = "scriptsDirectoryLabel";
        scriptsDirectoryLabel.Padding = new Padding(0, 6, 0, 0);
        scriptsDirectoryLabel.Size = new Size(100, 21);
        scriptsDirectoryLabel.TabIndex = 4;
        scriptsDirectoryLabel.Text = "Scripts Directory";
        //
        // sshKeyPathLabel
        //
        sshKeyPathLabel.Anchor = AnchorStyles.Left;
        sshKeyPathLabel.AutoSize = true;
        sshKeyPathLabel.Location = new Point(15, 105);
        sshKeyPathLabel.Name = "sshKeyPathLabel";
        sshKeyPathLabel.Padding = new Padding(0, 6, 0, 0);
        sshKeyPathLabel.Size = new Size(100, 21);
        sshKeyPathLabel.TabIndex = 6;
        sshKeyPathLabel.Text = "SSH Key Path";
        //
        // discordTokenLabel
        //
        discordTokenLabel.Anchor = AnchorStyles.Left;
        discordTokenLabel.AutoSize = true;
        discordTokenLabel.Location = new Point(15, 134);
        discordTokenLabel.Name = "discordTokenLabel";
        discordTokenLabel.Padding = new Padding(0, 6, 0, 0);
        discordTokenLabel.Size = new Size(100, 21);
        discordTokenLabel.TabIndex = 8;
        discordTokenLabel.Text = "Discord Token";
        //
        // discordGuildLabel
        //
        discordGuildLabel.Anchor = AnchorStyles.Left;
        discordGuildLabel.AutoSize = true;
        discordGuildLabel.Location = new Point(15, 163);
        discordGuildLabel.Name = "discordGuildLabel";
        discordGuildLabel.Padding = new Padding(0, 6, 0, 0);
        discordGuildLabel.Size = new Size(100, 21);
        discordGuildLabel.TabIndex = 10;
        discordGuildLabel.Text = "Discord Guild ID";
        //
        // discordChannelLabel
        //
        discordChannelLabel.Anchor = AnchorStyles.Left;
        discordChannelLabel.AutoSize = true;
        discordChannelLabel.Location = new Point(15, 192);
        discordChannelLabel.Name = "discordChannelLabel";
        discordChannelLabel.Padding = new Padding(0, 6, 0, 0);
        discordChannelLabel.Size = new Size(100, 21);
        discordChannelLabel.TabIndex = 12;
        discordChannelLabel.Text = "Discord Channel ID";
        //
        // discordRoleLabel
        //
        discordRoleLabel.Anchor = AnchorStyles.Left;
        discordRoleLabel.AutoSize = true;
        discordRoleLabel.Location = new Point(15, 221);
        discordRoleLabel.Name = "discordRoleLabel";
        discordRoleLabel.Padding = new Padding(0, 6, 0, 0);
        discordRoleLabel.Size = new Size(100, 21);
        discordRoleLabel.TabIndex = 14;
        discordRoleLabel.Text = "Discord Role ID";
        //
        // statusChannelLabel
        //
        statusChannelLabel.Anchor = AnchorStyles.Left;
        statusChannelLabel.AutoSize = true;
        statusChannelLabel.Location = new Point(15, 250);
        statusChannelLabel.Name = "statusChannelLabel";
        statusChannelLabel.Padding = new Padding(0, 6, 0, 0);
        statusChannelLabel.Size = new Size(100, 21);
        statusChannelLabel.TabIndex = 16;
        statusChannelLabel.Text = "Status Channel ID";
        //
        // githubRepoLabel
        //
        githubRepoLabel.Anchor = AnchorStyles.Left;
        githubRepoLabel.AutoSize = true;
        githubRepoLabel.Location = new Point(15, 279);
        githubRepoLabel.Name = "githubRepoLabel";
        githubRepoLabel.Padding = new Padding(0, 6, 0, 0);
        githubRepoLabel.Size = new Size(100, 21);
        githubRepoLabel.TabIndex = 18;
        githubRepoLabel.Text = "GitHub Repo";
        //
        // portBox
        //
        portBox.Location = new Point(180, 15);
        portBox.Maximum = new decimal(new int[] { 65535, 0, 0, 0 });
        portBox.Minimum = new decimal(new int[] { 1, 0, 0, 0 });
        portBox.Name = "portBox";
        portBox.Size = new Size(120, 23);
        portBox.TabIndex = 1;
        portBox.Value = new decimal(new int[] { 7374, 0, 0, 0 });
        //
        // vmNameBox
        //
        vmNameBox.Location = new Point(180, 44);
        vmNameBox.Name = "vmNameBox";
        vmNameBox.Size = new Size(360, 23);
        vmNameBox.TabIndex = 3;
        //
        // scriptsDirBox
        //
        scriptsDirBox.Location = new Point(180, 73);
        scriptsDirBox.Name = "scriptsDirBox";
        scriptsDirBox.Size = new Size(360, 23);
        scriptsDirBox.TabIndex = 5;
        //
        // sshKeyBox
        //
        sshKeyBox.Location = new Point(180, 102);
        sshKeyBox.Name = "sshKeyBox";
        sshKeyBox.Size = new Size(360, 23);
        sshKeyBox.TabIndex = 7;
        //
        // discordTokenBox
        //
        discordTokenBox.Location = new Point(180, 131);
        discordTokenBox.Name = "discordTokenBox";
        discordTokenBox.Size = new Size(360, 23);
        discordTokenBox.TabIndex = 9;
        discordTokenBox.UseSystemPasswordChar = true;
        //
        // discordGuildBox
        //
        discordGuildBox.Location = new Point(180, 160);
        discordGuildBox.Name = "discordGuildBox";
        discordGuildBox.Size = new Size(360, 23);
        discordGuildBox.TabIndex = 11;
        //
        // discordChannelBox
        //
        discordChannelBox.Location = new Point(180, 189);
        discordChannelBox.Name = "discordChannelBox";
        discordChannelBox.Size = new Size(360, 23);
        discordChannelBox.TabIndex = 13;
        //
        // discordRoleBox
        //
        discordRoleBox.Location = new Point(180, 218);
        discordRoleBox.Name = "discordRoleBox";
        discordRoleBox.Size = new Size(360, 23);
        discordRoleBox.TabIndex = 15;
        //
        // discordStatusChannelBox
        //
        discordStatusChannelBox.Location = new Point(180, 247);
        discordStatusChannelBox.Name = "discordStatusChannelBox";
        discordStatusChannelBox.Size = new Size(360, 23);
        discordStatusChannelBox.TabIndex = 17;
        //
        // githubRepoBox
        //
        githubRepoBox.Location = new Point(180, 276);
        githubRepoBox.Name = "githubRepoBox";
        githubRepoBox.Size = new Size(360, 23);
        githubRepoBox.TabIndex = 19;
        //
        // buttonPanel
        //
        buttonPanel.Controls.Add(cancelButton);
        buttonPanel.Controls.Add(saveButton);
        buttonPanel.Dock = DockStyle.Bottom;
        buttonPanel.FlowDirection = FlowDirection.RightToLeft;
        buttonPanel.Location = new Point(0, 437);
        buttonPanel.Name = "buttonPanel";
        buttonPanel.Padding = new Padding(8);
        buttonPanel.Size = new Size(604, 44);
        buttonPanel.TabIndex = 1;
        //
        // saveButton
        //
        saveButton.DialogResult = DialogResult.OK;
        saveButton.Location = new Point(413, 11);
        saveButton.Name = "saveButton";
        saveButton.Size = new Size(90, 23);
        saveButton.TabIndex = 1;
        saveButton.Text = "Save";
        saveButton.UseVisualStyleBackColor = true;
        //
        // cancelButton
        //
        cancelButton.DialogResult = DialogResult.Cancel;
        cancelButton.Location = new Point(509, 11);
        cancelButton.Name = "cancelButton";
        cancelButton.Size = new Size(75, 23);
        cancelButton.TabIndex = 0;
        cancelButton.Text = "Cancel";
        cancelButton.UseVisualStyleBackColor = true;
        //
        // SettingsForm
        //
        AcceptButton = saveButton;
        AutoScaleDimensions = new SizeF(7F, 15F);
        AutoScaleMode = AutoScaleMode.Font;
        CancelButton = cancelButton;
        ClientSize = new Size(604, 481);
        Controls.Add(settingsTable);
        Controls.Add(buttonPanel);
        FormBorderStyle = FormBorderStyle.FixedDialog;
        MaximizeBox = false;
        MinimizeBox = false;
        Name = "SettingsForm";
        StartPosition = FormStartPosition.CenterParent;
        Text = "Settings";
        settingsTable.ResumeLayout(false);
        settingsTable.PerformLayout();
        ((System.ComponentModel.ISupportInitialize)portBox).EndInit();
        buttonPanel.ResumeLayout(false);
        ResumeLayout(false);
    }

}
