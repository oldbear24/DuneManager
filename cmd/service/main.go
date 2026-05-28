package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/oldbear24/DuneManager/internal/api"
	"github.com/oldbear24/DuneManager/internal/config"
	"github.com/oldbear24/DuneManager/internal/discord"
	"github.com/oldbear24/DuneManager/internal/logging"
	"github.com/oldbear24/DuneManager/internal/winsvc"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/eventlog"
	"golang.org/x/sys/windows/svc/mgr"
)

const (
	svcName        = winsvc.ServiceName
	svcDisplayName = "Dune Awakening Server Manager"
	svcDescription = "Manages a Dune Awakening dedicated server running in Hyper-V"
)

func main() {
	config.Init()

	runFlag := flag.Bool("run", false, "Run the service in the foreground (no SCM)")
	installFlag := flag.Bool("install", false, "Install as a Windows service")
	removeFlag := flag.Bool("uninstall", false, "Uninstall the Windows service")
	startFlag := flag.Bool("start", false, "Start the Windows service")
	stopFlag := flag.Bool("stop", false, "Stop the Windows service")
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
	cleanup, err := configureLogging(true, nil)
	if err != nil {
		log.Fatalf("configure logging failed: %v", err)
	}
	defer cleanup()

	srv := api.NewServer()
	logging.Infof("service starting in foreground on %s", config.ServiceAddr())
	bot := startDiscordBot()
	defer func() {
		if bot != nil {
			logging.Infof("stopping Discord bot")
			bot.Stop()
		}
	}()
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logging.Errorf("service exited with error: %v", err)
		os.Exit(1)
	}
	logging.Infof("service stopped")
}

// ── Windows service handler ───────────────────────────────────────────────────

type duneService struct{ srv *api.Server }

func (ds *duneService) Execute(_ []string, r <-chan svc.ChangeRequest, changes chan<- svc.Status) (bool, uint32) {
	const cmdsAccepted = svc.AcceptStop | svc.AcceptShutdown
	changes <- svc.Status{State: svc.StartPending}

	ds.srv = api.NewServer()
	go func() {
		if err := ds.srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logging.Errorf("service listener failed: %v", err)
		}
	}()

	bot := startDiscordBot()
	logging.Infof("service started")

	changes <- svc.Status{State: svc.Running, Accepts: cmdsAccepted}

	for c := range r {
		switch c.Cmd {
		case svc.Interrogate:
			changes <- c.CurrentStatus
		case svc.Stop, svc.Shutdown:
			logging.Infof("service stop requested")
			changes <- svc.Status{State: svc.StopPending}
			if bot != nil {
				logging.Infof("stopping Discord bot")
				bot.Stop()
			}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			if err := ds.srv.Shutdown(ctx); err != nil {
				logging.Errorf("service shutdown failed: %v", err)
			} else {
				logging.Infof("service shutdown complete")
			}
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
		logging.Infof("Discord bot disabled")
		return nil
	}
	bot, err := discord.New(cfg.DiscordToken, cfg.DiscordGuildID, cfg.DiscordChannelID, cfg.DiscordRoleID)
	if err != nil {
		logging.Errorf("Discord bot failed to create session: %v", err)
		return nil
	}
	if err := bot.Start(); err != nil {
		logging.Errorf("Discord bot failed to start: %v", err)
		return nil
	}
	logging.Infof("Discord bot started")
	return bot
}

func runAsService() {
	elog, _ := eventlog.Open(svcName)
	if elog != nil {
		defer elog.Close()
	}
	var sink logging.EventSink
	if elog != nil {
		sink = windowsEventSink{elog: elog}
	}
	cleanup, err := configureLogging(false, sink)
	if err != nil {
		if elog != nil {
			_ = elog.Error(1, fmt.Sprintf("configure logging failed: %v", err))
		}
		os.Exit(1)
	}
	defer cleanup()
	logging.Infof("service starting under SCM")
	if elog != nil {
		_ = elog.Info(1, "service starting")
	}
	if err := svc.Run(svcName, &duneService{}); err != nil {
		logging.Errorf("service run failed: %v", err)
		os.Exit(1)
	}
	logging.Infof("service stopped")
	if elog != nil {
		_ = elog.Info(1, "service stopped")
	}
}

type windowsEventSink struct{ elog *eventlog.Log }

func (s windowsEventSink) Warning(msg string) {
	_ = s.elog.Warning(1, msg)
}

func (s windowsEventSink) Error(msg string) {
	_ = s.elog.Error(1, msg)
}

func configureLogging(mirrorStdout bool, sink logging.EventSink) (func(), error) {
	logFile, err := logging.Setup(logging.LogPath(), mirrorStdout)
	if err != nil {
		return nil, err
	}
	logging.SetEventSink(sink)
	return func() {
		logging.SetEventSink(nil)
		_ = logFile.Close()
	}, nil
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
	return winsvc.Start()
}

func stopService() error {
	return winsvc.Stop()
}
