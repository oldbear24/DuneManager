using System.Diagnostics;

namespace DuneManager.Backend;

internal sealed class ProcessRunner
{
    private readonly object gate = new();
    private Process? activeProcess;

    public bool HasActiveProcess
    {
        get
        {
            lock (gate)
            {
                return activeProcess is { HasExited: false };
            }
        }
    }

    public void KillActive()
    {
        lock (gate)
        {
            try
            {
                if (activeProcess is { HasExited: false })
                {
                    activeProcess.Kill(entireProcessTree: true);
                }
            }
            catch
            {
                // The process may have exited between the status check and kill request.
            }
        }
    }

    public async Task<int> RunAsync(string fileName, string arguments, Action<string> output, CancellationToken cancellationToken = default)
    {
        var startInfo = new ProcessStartInfo(fileName, arguments)
        {
            UseShellExecute = false,
            CreateNoWindow = true,
            RedirectStandardOutput = true,
            RedirectStandardError = true
        };

        using var process = new Process { StartInfo = startInfo, EnableRaisingEvents = true };
        lock (gate)
        {
            activeProcess = process;
        }

        process.OutputDataReceived += (_, e) =>
        {
            if (e.Data is null) return;
            try { output(e.Data + Environment.NewLine); }
            catch { /* ignore output sink failures (e.g., client disconnect) */ }
        };
        process.ErrorDataReceived += (_, e) =>
        {
            if (e.Data is null) return;
            try { output(e.Data + Environment.NewLine); }
            catch { /* ignore output sink failures (e.g., client disconnect) */ }
        };

        try
        {
            if (!process.Start())
            {
                throw new InvalidOperationException($"Could not start {fileName}.");
            }

            process.BeginOutputReadLine();
            process.BeginErrorReadLine();
            await process.WaitForExitAsync(cancellationToken);
            return process.ExitCode;
        }
        finally
        {
            lock (gate)
            {
                if (ReferenceEquals(activeProcess, process))
                {
                    activeProcess = null;
                }
            }
        }
    }

    public static async Task<string> CaptureAsync(string fileName, string arguments, CancellationToken cancellationToken = default)
    {
        var startInfo = new ProcessStartInfo(fileName, arguments)
        {
            UseShellExecute = false,
            CreateNoWindow = true,
            RedirectStandardOutput = true,
            RedirectStandardError = true
        };
        using var process = Process.Start(startInfo) ?? throw new InvalidOperationException($"Could not start {fileName}.");
        var stdout = await process.StandardOutput.ReadToEndAsync(cancellationToken);
        var stderr = await process.StandardError.ReadToEndAsync(cancellationToken);
        await process.WaitForExitAsync(cancellationToken);
        return stdout + stderr;
    }
}
