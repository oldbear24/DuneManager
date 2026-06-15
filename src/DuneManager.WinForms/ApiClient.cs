using System.Net.Http.Json;
using System.Text;
using System.Text.Json;
using DuneManager.Shared;

namespace DuneManager.WinForms;

internal sealed class ApiClient : IDisposable
{
    private readonly HttpClient http = new() { Timeout = Timeout.InfiniteTimeSpan };
    private readonly JsonSerializerOptions jsonOptions = new() { PropertyNamingPolicy = JsonNamingPolicy.CamelCase };

    public ApiClient(DuneConfig config)
    {
        BaseUrl = $"http://127.0.0.1:{config.Port}";
    }

    public string BaseUrl { get; }

    public async Task<StatusResponse?> GetStatusAsync(CancellationToken cancellationToken = default)
    {
        try
        {
            using var timeout = CancellationTokenSource.CreateLinkedTokenSource(cancellationToken);
            timeout.CancelAfter(TimeSpan.FromSeconds(3));
            return await http.GetFromJsonAsync<StatusResponse>(BaseUrl + "/api/status", timeout.Token);
        }
        catch
        {
            return null;
        }
    }

    public async Task<string> ExecAsync(ExecRequest request, Action<string> onOutput, CancellationToken cancellationToken = default)
    {
        using var response = await http.PostAsJsonAsync(BaseUrl + "/api/exec", request, jsonOptions, cancellationToken);
        response.EnsureSuccessStatusCode();
        await using var stream = await response.Content.ReadAsStreamAsync(cancellationToken);
        using var reader = new StreamReader(stream, Encoding.UTF8);
        string result = string.Empty;
        string error = string.Empty;

        while (!reader.EndOfStream && !cancellationToken.IsCancellationRequested)
        {
            var line = await reader.ReadLineAsync(cancellationToken);
            if (line is null || !line.StartsWith("data: ", StringComparison.Ordinal))
            {
                continue;
            }

            var evt = JsonSerializer.Deserialize<SseEvent>(line[6..], jsonOptions);
            if (evt is null)
            {
                continue;
            }

            if (evt.Type == "output")
            {
                onOutput(evt.Line);
            }
            else if (evt.Type == "done")
            {
                result = evt.Line;
                error = evt.Error;
            }
        }

        if (!string.IsNullOrWhiteSpace(error))
        {
            throw new InvalidOperationException(error);
        }

        return result;
    }

    public async Task KillAsync(CancellationToken cancellationToken = default)
    {
        using var response = await http.PostAsync(BaseUrl + "/api/kill", null, cancellationToken);
        response.EnsureSuccessStatusCode();
    }

    public async Task<UpdateCheckResponse> CheckUpdateAsync(CancellationToken cancellationToken = default)
    {
        return await http.GetFromJsonAsync<UpdateCheckResponse>(BaseUrl + "/api/update/check", cancellationToken) ?? new UpdateCheckResponse();
    }

    public void Dispose()
    {
        http.Dispose();
    }
}
