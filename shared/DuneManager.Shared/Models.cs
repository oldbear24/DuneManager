using System.Text.Json.Serialization;

namespace DuneManager.Shared;

public sealed class DuneConfig
{
    [JsonPropertyName("port")]
    public int Port { get; set; } = 7374;

    [JsonPropertyName("vmName")]
    public string VMName { get; set; } = "dune-awakening";

    [JsonPropertyName("scriptsDir")]
    public string ScriptsDir { get; set; } = DefaultScriptsDir();

    [JsonPropertyName("sshKeyPath")]
    public string SSHKeyPath { get; set; } = DefaultSSHKeyPath();

    [JsonPropertyName("discordToken")]
    public string DiscordToken { get; set; } = string.Empty;

    [JsonPropertyName("discordGuildID")]
    public string DiscordGuildID { get; set; } = string.Empty;

    [JsonPropertyName("discordChannelID")]
    public string DiscordChannelID { get; set; } = string.Empty;

    [JsonPropertyName("discordRoleID")]
    public string DiscordRoleID { get; set; } = string.Empty;

    [JsonPropertyName("discordStatusChannelID")]
    public string DiscordStatusChannelID { get; set; } = string.Empty;

    [JsonPropertyName("discordStatusMsgID")]
    public string DiscordStatusMsgID { get; set; } = string.Empty;

    [JsonPropertyName("githubRepo")]
    public string GitHubRepo { get; set; } = "oldbear24/DuneManager";

    public static string DefaultPath()
    {
        var exeDir = AppContext.BaseDirectory;
        return Path.Combine(exeDir, "dune-manager.json");
    }

    public static string DefaultSSHKeyPath()
    {
        var local = Environment.GetFolderPath(Environment.SpecialFolder.LocalApplicationData);
        return Path.Combine(local, "DuneAwakeningServer", "sshKey");
    }

    public static string DefaultScriptsDir()
    {
        return Path.Combine(AppContext.BaseDirectory, "bats", "battlegroup-management");
    }
}

public sealed class ExecRequest
{
    [JsonPropertyName("cmd")]
    public string Cmd { get; set; } = string.Empty;

    [JsonPropertyName("password")]
    public string Password { get; set; } = string.Empty;
}

public sealed class StatusResponse
{
    [JsonPropertyName("exists")]
    public bool Exists { get; set; }

    [JsonPropertyName("running")]
    public bool Running { get; set; }

    [JsonPropertyName("vmState")]
    public string VMState { get; set; } = string.Empty;

    [JsonPropertyName("ip")]
    public string IP { get; set; } = string.Empty;

    [JsonPropertyName("busy")]
    public bool Busy { get; set; }
}

public sealed class SseEvent
{
    [JsonPropertyName("type")]
    public string Type { get; set; } = string.Empty;

    [JsonPropertyName("line")]
    public string Line { get; set; } = string.Empty;

    [JsonPropertyName("error")]
    public string Error { get; set; } = string.Empty;
}

public sealed class VersionResponse
{
    [JsonPropertyName("version")]
    public string Version { get; set; } = "dev-dotnet";
}

public sealed class UpdateCheckResponse
{
    [JsonPropertyName("current")]
    public string Current { get; set; } = "dev-dotnet";

    [JsonPropertyName("latest")]
    public string Latest { get; set; } = string.Empty;

    [JsonPropertyName("hasUpdate")]
    public bool HasUpdate { get; set; }

    [JsonPropertyName("error")]
    public string Error { get; set; } = string.Empty;
}
