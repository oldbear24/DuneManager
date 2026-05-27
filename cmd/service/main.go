package main

import (
"context"
"flag"
"fmt"
"log"
"os"
"time"

"dune-manager/internal/api"
"dune-manager/internal/config"
"dune-manager/internal/discord"
"golang.org/x/sys/windows/svc"
"golang.org/x/sys/windows/svc/eventlog"
"golang.org/x/sys/windows/svc/mgr"
)

const (
svcName        = "DuneManager"
svcDisplayName = "Dune Awakening Server Manager"
svcDescription = "Manages a Dune Awakening dedicated server running in Hyper-V"
)

func main() {
config.Init()

runFlag     := flag.Bool("run",       false, "Run the service in the foreground (no SCM)")
installFlag := flag.Bool("install",   false, "Install as a Windows service")
removeFlag  := flag.Bool("uninstall", false, "Uninstall the Windows service")
startFlag   := flag.Bool("start",     false, "Start the Windows service")
stopFlag    := flag.Bool("stop",      false, "Stop the Windows service")
flag.Parse()

switch {
case *installFlag:
mustOK(installService(), "install")
fmt.Println("Service installed.")
case *removeFlag:
mustOK(removeService(), "uninstall")
fmt.Println("Service removed.")
case *startFlag:
mustOK(startService(), "start")
fmt.Println("Service started.")
case *stopFlag:
mustOK(stopService(), "stop")
fmt.Println("Service stopped.")
case *runFlag:
runForeground()
default:
isService, err := svc.IsWindowsService()
if err != nil || !isService {
fmt.Fprintln(os.Stderr,
"Usage: dune-manager-svc.exe --run | --install | --uninstall | --start | --stop")
os.Exit(1)
}
runAsService()
}
}

func mustOK(err error, op string) {
if err != nil {
log.Fatalf("%s failed: %v", op, err)
}
}

// runForeground starts the HTTP service (and optional Discord bot) and blocks.
func runForeground() {
srv := api.NewServer()
fmt.Printf("Dune Manager service listening on http://%s\n", config.ServiceAddr())
bot := startDiscordBot()
defer func() {
if bot != nil {
	bot.Stop()
}
}()
if err := srv.ListenAndServe(); err != nil {
log.Fatal(err)
}
}

// ── Windows service handler ───────────────────────────────────────────────────

type duneService struct{ srv *api.Server }

func (ds *duneService) Execute(_ []string, r <-chan svc.ChangeRequest, changes chan<- svc.Status) (bool, uint32) {
const cmdsAccepted = svc.AcceptStop | svc.AcceptShutdown
changes <- svc.Status{State: svc.StartPending}

ds.srv = api.NewServer()
go func() { _ = ds.srv.ListenAndServe() }()

bot := startDiscordBot()

changes <- svc.Status{State: svc.Running, Accepts: cmdsAccepted}

for c := range r {
switch c.Cmd {
case svc.Interrogate:
changes <- c.CurrentStatus
case svc.Stop, svc.Shutdown:
changes <- svc.Status{State: svc.StopPending}
if bot != nil {
	bot.Stop()
}
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
_ = ds.srv.Shutdown(ctx)
cancel()
return false, 0
}
}
return false, 0
}

// startDiscordBot starts the Discord bot if a token is configured.
// Returns nil (and logs a warning) if startup fails or no token is set.
func startDiscordBot() *discord.Bot {
cfg := config.Get()
if cfg.DiscordToken == "" {
return nil
}
bot, err := discord.New(cfg.DiscordToken, cfg.DiscordGuildID, cfg.DiscordChannelID)
if err != nil {
log.Printf("Discord bot: failed to create session: %v", err)
return nil
}
if err := bot.Start(); err != nil {
log.Printf("Discord bot: failed to start: %v", err)
return nil
}
log.Println("Discord bot started.")
return bot
}

func runAsService() {
elog, _ := eventlog.Open(svcName)
if elog != nil {
defer elog.Close()
}
if err := svc.Run(svcName, &duneService{}); err != nil {
if elog != nil {
_ = elog.Error(1, fmt.Sprintf("service run failed: %v", err))
}
os.Exit(1)
}
}

// ── Service management helpers ────────────────────────────────────────────────

func installService() error {
exePath, err := os.Executable()
if err != nil {
return err
}
m, err := mgr.Connect()
if err != nil {
return err
}
defer m.Disconnect()

if s, err := m.OpenService(svcName); err == nil {
s.Close()
return fmt.Errorf("service %q already exists", svcName)
}

s, err := m.CreateService(svcName, exePath, mgr.Config{
DisplayName: svcDisplayName,
Description: svcDescription,
StartType:   mgr.StartAutomatic,
})
if err != nil {
return err
}
defer s.Close()
_ = eventlog.InstallAsEventCreate(svcName, eventlog.Error|eventlog.Warning|eventlog.Info)
return nil
}

func removeService() error {
m, err := mgr.Connect()
if err != nil {
return err
}
defer m.Disconnect()

s, err := m.OpenService(svcName)
if err != nil {
return fmt.Errorf("service %q not found", svcName)
}
defer s.Close()
if err := s.Delete(); err != nil {
return err
}
_ = eventlog.Remove(svcName)
return nil
}

func startService() error {
m, err := mgr.Connect()
if err != nil {
return err
}
defer m.Disconnect()

s, err := m.OpenService(svcName)
if err != nil {
return fmt.Errorf("service %q not found", svcName)
}
defer s.Close()
return s.Start()
}

func stopService() error {
m, err := mgr.Connect()
if err != nil {
return err
}
defer m.Disconnect()

s, err := m.OpenService(svcName)
if err != nil {
return fmt.Errorf("service %q not found", svcName)
}
defer s.Close()
_, err = s.Control(svc.Stop)
return err
}
