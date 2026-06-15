using DuneManager.Shared;

namespace DuneManager.WinForms;

internal sealed partial class SettingsForm : Form
{
    public SettingsForm()
    {
        InitializeComponent();
    }

    public SettingsForm(DuneConfig config) : this()
    {
        LoadConfig(config);
    }

    public DuneConfig ToConfig(DuneConfig existing)
    {
        return new DuneConfig
        {
            Port = (int)portBox.Value,
            VMName = vmNameBox.Text.Trim(),
            ScriptsDir = scriptsDirBox.Text.Trim(),
            SSHKeyPath = sshKeyBox.Text.Trim(),
            DiscordToken = discordTokenBox.Text,
            DiscordGuildID = discordGuildBox.Text.Trim(),
            DiscordChannelID = discordChannelBox.Text.Trim(),
            DiscordRoleID = discordRoleBox.Text.Trim(),
            DiscordStatusChannelID = discordStatusChannelBox.Text.Trim(),
            DiscordStatusMsgID = discordStatusChannelBox.Text.Trim() == existing.DiscordStatusChannelID ? existing.DiscordStatusMsgID : string.Empty,
            GitHubRepo = githubRepoBox.Text.Trim()
        };
    }

    private void LoadConfig(DuneConfig config)
    {
        portBox.Value = config.Port;
        vmNameBox.Text = config.VMName;
        scriptsDirBox.Text = config.ScriptsDir;
        sshKeyBox.Text = config.SSHKeyPath;
        discordTokenBox.Text = config.DiscordToken;
        discordGuildBox.Text = config.DiscordGuildID;
        discordChannelBox.Text = config.DiscordChannelID;
        discordRoleBox.Text = config.DiscordRoleID;
        discordStatusChannelBox.Text = config.DiscordStatusChannelID;
        githubRepoBox.Text = config.GitHubRepo;
    }
}
