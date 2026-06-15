using System.Diagnostics;
using System.Net;
using System.Text;
using System.Text.Json;
using System.Text.RegularExpressions;
using DuneManager.Shared;

namespace DuneManager.Backend;

internal sealed class BackendServer
{
    private readonly ConfigStore configStore;
    private readonly HttpListener listener = new();
    private readonly ProcessRunner runner = new();
    private readonly SemaphoreSlim exclusive = new(1, 1);
    private readonly JsonSerializerOptions jsonOptions = new() { PropertyNamingPolicy = JsonNamingPolicy.CamelCase };
    private readonly CancellationTokenSource shutdown = new();
    private volatile bool busy;

    public BackendServer(ConfigStore configStore)
    {
        this.configStore = configStore;
        var cfg = configStore.Snapshot();
        listener.Prefixes.Add($"http://127.0.0.1:{cfg.Port}/");
        listener.Prefixes.Add($"http://localhost:{cfg.Port}/");
    }

    public async Task RunAsync()
    {
        listener.Start();
        Console.WriteLine($"Dune Manager backend listening on {string.Join(", ", listener.Prefixes)}");
        while (!shutdown.IsCancellationRequested)
        {
            HttpListenerContext context;
            try
            {
                context = await listener.GetContextAsync();
            }
            catch when (shutdown.IsCancellationRequested || !listener.IsListening)
            {
                break;
            }

            _ = Task.Run(() => RouteAsync(context));
        }
    }

    public void Stop()
    {
        shutdown.Cancel();
        listener.Stop();
    }

    private async Task RouteAsync(HttpListenerContext context)
    {
        try
        {
            var path = context.Request.Url?.AbsolutePath ?? "/";
            switch (path)
            {
                case "/api/status":
                    await WriteJsonAsync(context, await GetStatusAsync());
                    break;
                case "/api/exec":
                    await HandleExecAsync(context);
                    break;
                case "/api/kill":
                    runner.KillActive();
                    context.Response.StatusCode = (int)HttpStatusCode.NoContent;
                    context.Response.Close();
                    break;
                case "/api/version":
                    await WriteJsonAsync(context, new VersionResponse());
                    break;
                case "/api/update/check":
                    await WriteJsonAsync(context, new UpdateCheckResponse { Error = "Auto-update is not implemented in the .NET rewrite yet." });
                    break;
                case "/api/service/restart":
                    await WriteJsonAsync(context, new { status = "backend console apps should be restarted by their host process" });
                    break;
                default:
                    context.Response.StatusCode = (int)HttpStatusCode.NotFound;
                    await WriteTextAsync(context, "not found");
                    break;
            }
        }
        catch (Exception ex)
        {
            if (context.Response.OutputStream.CanWrite)
            {
                context.Response.StatusCode = (int)HttpStatusCode.InternalServerError;
                await WriteTextAsync(context, ex.Message);
            }
        }
    }

    private async Task<StatusResponse> GetStatusAsync()
    {
        var cfg = configStore.Snapshot();
        var escapedName = EscapePowerShellSingleQuoted(cfg.VMName);
        var script = "$vm = Get-VM -Name '" + escapedName + "' -ErrorAction SilentlyContinue; " +
                     "if (-not $vm) { 'missing||' } else { " +
                     "$ip = (Get-VMNetworkAdapter -VMName '" + escapedName + "').IPAddresses | Where-Object { $_ -match '^\\d+\\.\\d+\\.\\d+\\.\\d+$' } | Select-Object -First 1; " +
                     "$exists = 'exists'; $running = if ($vm.State -eq 'Running') { 'true' } else { 'false' }; " +
                     "\"$exists|$($vm.State)|$running|$ip\" }";
        var output = (await ProcessRunner.CaptureAsync("powershell", PowerShellArgs(script))).Trim();
        var parts = output.Split('|');

        if (parts.Length < 4 || parts[0] != "exists")
        {
            return new StatusResponse { Exists = false, VMState = "missing", Busy = busy };
        }

        return new StatusResponse
        {
            Exists = true,
            VMState = parts[1],
            Running = parts[2].Equals("true", StringComparison.OrdinalIgnoreCase),
            IP = parts[3],
            Busy = busy
        };
    }

    private async Task HandleExecAsync(HttpListenerContext context)
    {
        if (!context.Request.HttpMethod.Equals("POST", StringComparison.OrdinalIgnoreCase))
        {
            context.Response.StatusCode = (int)HttpStatusCode.MethodNotAllowed;
            await WriteTextAsync(context, "POST required");
            return;
        }

        var request = await JsonSerializer.DeserializeAsync<ExecRequest>(context.Request.InputStream, jsonOptions) ?? new ExecRequest();
        context.Response.StatusCode = (int)HttpStatusCode.OK;
        context.Response.ContentType = "text/event-stream";
        context.Response.Headers["Cache-Control"] = "no-cache";

        if (!await exclusive.WaitAsync(0))
        {
            WriteSseBlocking(context, new SseEvent { Type = "done", Error = "busy: another command is running" });
            context.Response.Close();
            return;
        }

        busy = true;
        try
        {
            string result = string.Empty;
            await ExecuteCommandAsync(request, line => WriteSseBlocking(context, new SseEvent { Type = "output", Line = StripAnsi(line) }), value => result = value);
            WriteSseBlocking(context, new SseEvent { Type = "done", Line = result });
        }
        catch (Exception ex)
        {
            WriteSseBlocking(context, new SseEvent { Type = "done", Error = ex.Message });
        }
        finally
        {
            busy = false;
            exclusive.Release();
            context.Response.Close();
        }
    }

    private async Task ExecuteCommandAsync(ExecRequest request, Action<string> output, Action<string> setResult)
    {
        var cfg = configStore.Snapshot();
        var state = await GetStatusAsync();

        switch (request.Cmd)
        {
            case "vm-start":
                await RunPowerShellAsync(StartVmScript(cfg.VMName), output);
                break;
            case "vm-stop":
                await RunPowerShellAsync($"Stop-VM -Name '{EscapePowerShellSingleQuoted(cfg.VMName)}' -Force; Write-Host 'VM stopped.'", output);
                break;
            case "ssh-rotate":
                await RotateSshKeyAsync(cfg, state, request.Password, output);
                break;
            case "password-change":
                RequireVm(state);
                await RunSshAsync(cfg, state, $"echo 'dune:{EscapeShellSingleQuoted(request.Password)}' | sudo chpasswd", output);
                output("Password changed.\n");
                break;
            case "bg-status":
            case "bg-start":
            case "bg-stop":
            case "bg-restart":
            case "bg-update":
            case "bg-backup":
            case "bg-swap":
                var subcommand = request.Cmd[3..] == "swap" ? "enable-experimental-swap" : request.Cmd[3..];
                await RunBattlegroupAsync(cfg, state, subcommand, output);
                break;
            case "director-port":
                var port = await CaptureSshAsync(cfg, state, "sudo kubectl get svc -A -o jsonpath='{.items[*].spec.ports[?(@.port==11717)].nodePort}' 2>&1", output);
                var cleanPort = Regex.Replace(port, "\\s+", string.Empty);
                if (!Regex.IsMatch(cleanPort, "^\\d+$"))
                {
                    throw new InvalidOperationException("Could not determine Director port - is the battlegroup running?");
                }
                setResult($"http://{state.IP}:{cleanPort}/");
                break;
            default:
                throw new InvalidOperationException($"unknown command: {request.Cmd}");
        }
    }

    private async Task RunBattlegroupAsync(DuneConfig cfg, StatusResponse state, string subcommand, Action<string> output)
    {
        RequireVm(state);
        var lines = new List<string>();
        try
        {
            await RunSshAsync(cfg, state, $"TERM=dumb /home/dune/.dune/bin/battlegroup {subcommand} 2>&1", line =>
            {
                lines.Add(line.ToLowerInvariant());
                output(line);
            });
        }
        catch when (subcommand == "update" && lines.Any(line => line.Contains("finished updating battlegroup") || line.Contains("already up to date") || line.Contains("finished loading battlegroup images")))
        {
            output("Update reported success; ignoring trailing symlink exit status.\n");
        }
    }

    private async Task RotateSshKeyAsync(DuneConfig cfg, StatusResponse state, string password, Action<string> output)
    {
        RequireVm(state);
        Directory.CreateDirectory(Path.GetDirectoryName(cfg.SSHKeyPath)!);
        var tempKey = Path.Combine(Path.GetTempPath(), "dune-newkey-" + Guid.NewGuid().ToString("N"));
        output("Generating new SSH key pair...\n");
        await RunProcessCheckedAsync("ssh-keygen", $"-t ed25519 -f \"{tempKey}\" -N \"\" -q", output);

        var publicKey = Convert.ToBase64String(await File.ReadAllBytesAsync(tempKey + ".pub"));
        var installCommand = "mkdir -p $HOME/.ssh && chmod 700 $HOME/.ssh && echo " + publicKey + " | base64 -d > $HOME/.ssh/authorized_keys && chmod 600 $HOME/.ssh/authorized_keys && echo ROTATE_OK";
        output("Installing new public key on VM...\n");
        try
        {
            await RunSshAsync(cfg, state, installCommand, output);
        }
        catch when (!string.IsNullOrWhiteSpace(password))
        {
            output("Key auth failed; retry password fallback with ssh-copy style command manually if needed.\n");
            throw;
        }

        File.Copy(tempKey, cfg.SSHKeyPath, overwrite: true);
        File.Copy(tempKey + ".pub", cfg.SSHKeyPath + ".pub", overwrite: true);
        File.Delete(tempKey);
        File.Delete(tempKey + ".pub");
        output($"SSH key rotated: {cfg.SSHKeyPath}\n");
    }

    private async Task<string> CaptureSshAsync(DuneConfig cfg, StatusResponse state, string command, Action<string> output)
    {
        var builder = new StringBuilder();
        await RunSshAsync(cfg, state, command, line =>
        {
            builder.Append(line);
            output(line);
        });
        return builder.ToString();
    }

    private async Task RunSshAsync(DuneConfig cfg, StatusResponse state, string command, Action<string> output)
    {
        RequireVm(state);
        await RunProcessCheckedAsync("ssh", $"-o StrictHostKeyChecking=no -i \"{cfg.SSHKeyPath}\" dune@{state.IP} \"{command.Replace("\"", "\\\"")}\"", output);
    }

    private async Task RunPowerShellAsync(string script, Action<string> output)
    {
        await RunProcessCheckedAsync("powershell", PowerShellArgs(script), output);
    }

    private async Task RunProcessCheckedAsync(string fileName, string arguments, Action<string> output)
    {
        var exitCode = await runner.RunAsync(fileName, arguments, output, shutdown.Token);
        if (exitCode != 0)
        {
            throw new InvalidOperationException($"{fileName} exited with code {exitCode}");
        }
    }

    private static void RequireVm(StatusResponse state)
    {
        if (!state.Running || string.IsNullOrWhiteSpace(state.IP))
        {
            throw new InvalidOperationException("VM is not running or IP is unavailable");
        }
    }

    private static string StartVmScript(string vmName)
    {
        var name = EscapePowerShellSingleQuoted(vmName);
        return $@"
$vmName = '{name}'
$vm = Get-VM -Name $vmName -ErrorAction SilentlyContinue
if (-not $vm) {{ Write-Host ""VM '$vmName' does not exist.""; exit 1 }}
if ($vm.State -eq 'Running') {{ Write-Host 'VM is already running.'; exit 0 }}
Write-Host ""Starting VM '$vmName'...""
Start-VM -Name $vmName
$timeout = 120; $elapsed = 0
do {{ Start-Sleep -Seconds 2; $elapsed += 2; $vm = Get-VM -Name $vmName }} while ($vm.State -ne 'Running' -and $elapsed -lt $timeout)
if ($vm.State -eq 'Running') {{ Write-Host 'VM started.' }} else {{ Write-Host 'VM did not reach Running state in time.'; exit 1 }}";
    }

    private async Task WriteJsonAsync(HttpListenerContext context, object value)
    {
        context.Response.ContentType = "application/json";
        var json = JsonSerializer.Serialize(value, jsonOptions);
        await WriteTextAsync(context, json);
    }

    private static async Task WriteTextAsync(HttpListenerContext context, string text)
    {
        var bytes = Encoding.UTF8.GetBytes(text);
        await context.Response.OutputStream.WriteAsync(bytes);
        context.Response.Close();
    }

    private void WriteSseBlocking(HttpListenerContext context, SseEvent evt)
    {
        WriteSseAsync(context, evt).GetAwaiter().GetResult();
    }

    private async Task WriteSseAsync(HttpListenerContext context, SseEvent evt)
    {
        var payload = "data: " + JsonSerializer.Serialize(evt, jsonOptions) + "\n\n";
        var bytes = Encoding.UTF8.GetBytes(payload);
        await context.Response.OutputStream.WriteAsync(bytes);
        await context.Response.OutputStream.FlushAsync();
    }

    private static string PowerShellArgs(string script) => $"-NoProfile -ExecutionPolicy Bypass -Command \"{script.Replace("\"", "\\\"")}\"";

    private static string EscapePowerShellSingleQuoted(string value) => value.Replace("'", "''");

    private static string EscapeShellSingleQuoted(string value) => value.Replace("'", "'\\''");

    private static string StripAnsi(string value) => Regex.Replace(value, @"\x1b\[[0-9;]*[a-zA-Z]|\x1b[^\[]?[a-zA-Z]|\x0f|\x0e", string.Empty);
}
