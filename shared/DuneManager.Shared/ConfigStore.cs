using System.Text.Json;

namespace DuneManager.Shared;

public sealed class ConfigStore
{
    private readonly object gate = new();
    private readonly JsonSerializerOptions jsonOptions = new() { WriteIndented = true };

    public ConfigStore(string? path = null)
    {
        Path = path ?? DuneConfig.DefaultPath();
        Current = LoadFromDisk();
    }

    public string Path { get; }

    public DuneConfig Current { get; private set; }

    public DuneConfig Snapshot()
    {
        lock (gate)
        {
            return new DuneConfig
            {
                Port = Current.Port,
                VMName = Current.VMName,
                ScriptsDir = Current.ScriptsDir,
                SSHKeyPath = Current.SSHKeyPath,
                DiscordToken = Current.DiscordToken,
                DiscordGuildID = Current.DiscordGuildID,
                DiscordChannelID = Current.DiscordChannelID,
                DiscordRoleID = Current.DiscordRoleID,
                DiscordStatusChannelID = Current.DiscordStatusChannelID,
                DiscordStatusMsgID = Current.DiscordStatusMsgID,
                GitHubRepo = Current.GitHubRepo
            };
        }
    }

    public void Save(DuneConfig config)
    {
        Normalize(config);
        lock (gate)
        {
            Directory.CreateDirectory(System.IO.Path.GetDirectoryName(Path)!);
            File.WriteAllText(Path, JsonSerializer.Serialize(config, jsonOptions));
            Current = config;
        }
    }

    private DuneConfig LoadFromDisk()
    {
        try
        {
            if (File.Exists(Path))
            {
                var loaded = JsonSerializer.Deserialize<DuneConfig>(File.ReadAllText(Path)) ?? new DuneConfig();
                Normalize(loaded);
                return loaded;
            }
        }
        catch
        {
            // Keep startup resilient; the settings dialog can overwrite bad JSON.
        }

        return new DuneConfig();
    }

    private static void Normalize(DuneConfig config)
    {
        if (config.Port is < 1 or > 65535)
        {
            config.Port = 7374;
        }
        if (string.IsNullOrWhiteSpace(config.VMName))
        {
            config.VMName = "dune-awakening";
        }
        if (string.IsNullOrWhiteSpace(config.ScriptsDir))
        {
            config.ScriptsDir = DuneConfig.DefaultScriptsDir();
        }
        if (string.IsNullOrWhiteSpace(config.SSHKeyPath))
        {
            config.SSHKeyPath = DuneConfig.DefaultSSHKeyPath();
        }
        if (string.IsNullOrWhiteSpace(config.GitHubRepo))
        {
            config.GitHubRepo = "oldbear24/DuneManager";
        }
    }
}
