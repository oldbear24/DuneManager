using DuneManager.Backend;

var config = new DuneManager.Shared.ConfigStore();
var server = new BackendServer(config);

Console.CancelKeyPress += (_, eventArgs) =>
{
    eventArgs.Cancel = true;
    server.Stop();
};

if (args.Contains("--help", StringComparer.OrdinalIgnoreCase))
{
    Console.WriteLine("Usage: dune-manager-backend.exe [--run]");
    Console.WriteLine("Runs the Dune Manager HTTP backend console app.");
    return;
}

await server.RunAsync();
