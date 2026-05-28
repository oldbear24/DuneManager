package discord

import (
	"fmt"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"

	"github.com/oldbear24/DuneManager/internal/api"
	"github.com/oldbear24/DuneManager/internal/config"
	"github.com/oldbear24/DuneManager/internal/logging"
)

const (
	pollInterval = time.Minute

	// Discord Components V2 limits.
	maxTextDisplay = 4000
)

// ── poller lifecycle ──────────────────────────────────────────────────────────

func (b *Bot) startPoller() {
	b.statusMsgID = config.Get().DiscordStatusMsgID
	b.pollerStop = make(chan struct{})
	go b.runPoller()
}

func (b *Bot) stopPoller() {
	if b.pollerStop != nil {
		close(b.pollerStop)
		b.pollerStop = nil
	}
}

func (b *Bot) runPoller() {
	b.pollOnce()
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			b.pollOnce()
		case <-b.pollerStop:
			return
		}
	}
}

// pollState holds the resolved status data for one poll cycle.
type pollState struct {
	serviceOnline bool
	vmRunning     bool
	bg            *bgStatus // nil when service offline or VM not running
	fetchErr      error     // error from bg-status command
}

func (b *Bot) pollOnce() {
	ps := b.fetchPollState()
	components := b.buildStatusComponents(ps)
	b.updatePresence(ps)

	if b.statusChanID == "" {
		return
	}

	if b.statusMsgID == "" {
		msg, err := b.dg.ChannelMessageSendComplex(b.statusChanID, &discordgo.MessageSend{
			Flags:      discordgo.MessageFlagsIsComponentsV2,
			Components: components,
		})
		if err != nil {
			logging.Errorf("poller: post status message: %v", err)
			return
		}
		b.statusMsgID = msg.ID
		logging.Infof("poller: posted status message %s", b.statusMsgID)
		if err := config.SaveStatusMsgID(b.statusMsgID); err != nil {
			logging.Warningf("poller: save status msg ID: %v", err)
		}
		return
	}

	edit := discordgo.NewMessageEdit(b.statusChanID, b.statusMsgID)
	edit.Flags = discordgo.MessageFlagsIsComponentsV2
	edit.Components = &components
	if _, err := b.dg.ChannelMessageEditComplex(edit); err != nil {
		logging.Warningf("poller: edit status message %s: %v — will repost", b.statusMsgID, err)
		b.statusMsgID = ""
		_ = config.SaveStatusMsgID("")
	}
}

// fetchPollState fetches all required data for one poll cycle.
func (b *Bot) fetchPollState() pollState {
	status, err := b.pollerClient.GetStatus()
	if err != nil {
		return pollState{}
	}
	if !status.Running {
		return pollState{serviceOnline: true, vmRunning: false}
	}
	bg, fetchErr := fetchParsedServers(b.pollerClient)
	return pollState{serviceOnline: true, vmRunning: true, bg: bg, fetchErr: fetchErr}
}

// updatePresence sets the bot's Discord presence based on current server state.
func (b *Bot) updatePresence(ps pollState) {
	var discordStatus string
	var activityName string

	switch {
	case !ps.serviceOnline:
		discordStatus = "dnd"
		activityName = "Service offline"
	case !ps.vmRunning:
		discordStatus = "dnd"
		activityName = "VM offline"
	case ps.fetchErr != nil:
		discordStatus = "idle"
		activityName = "Status unavailable"
	case ps.bg == nil || len(ps.bg.Servers) == 0:
		discordStatus = "idle"
		if ps.bg != nil && strings.EqualFold(ps.bg.Status, "stopped") {
			activityName = "Battlegroup stopped"
		} else {
			activityName = "No servers running"
		}
	default:
		n := len(ps.bg.Servers)
		discordStatus = "online"
		if n == 1 {
			activityName = "1 server running"
		} else {
			activityName = fmt.Sprintf("%d servers running", n)
		}
	}

	activityType := discordgo.ActivityTypeWatching
	if err := b.dg.UpdateStatusComplex(discordgo.UpdateStatusData{
		Status: discordStatus,
		Activities: []*discordgo.Activity{
			{Name: activityName, Type: activityType},
		},
	}); err != nil {
		logging.Warningf("poller: update presence: %v", err)
	}
}

// ── status message builder ────────────────────────────────────────────────────

func (b *Bot) buildStatusComponents(ps pollState) []discordgo.MessageComponent {
	now := time.Now().Format("02 Jan 2006 15:04:05 MST")

	footer := v2Text("-# 🕐 Updated: " + now)

	if !ps.serviceOnline {
		return wrapContainer(accentRed,
			v2Text("## 🏜️ Dune Awakening — Game Servers"),
			v2Sep(),
			v2Text("🔴 **Service offline**"),
			footer,
		)
	}

	if !ps.vmRunning {
		return wrapContainer(accentRed,
			v2Text("## 🏜️ Dune Awakening — Game Servers"),
			v2Sep(),
			v2Text("🔴 **Not Running**"),
			footer,
		)
	}

	comps := []discordgo.MessageComponent{
		v2Text("## 🏜️ Dune Awakening — Game Servers"),
		v2Sep(),
	}

	switch {
	case ps.fetchErr != nil && strings.Contains(ps.fetchErr.Error(), "busy"):
		comps = append(comps, v2Text("⏳ Command in progress…"))
	case ps.fetchErr != nil:
		comps = append(comps, v2Text("❌ "+ps.fetchErr.Error()))
	case ps.bg == nil || len(ps.bg.Servers) == 0:
		statusLine := bgStatusLine(func() string {
			if ps.bg != nil {
				return ps.bg.Status
			}
			return ""
		}())
		comps = append(comps, v2Text(statusLine))
		comps = append(comps, v2Sep(), footer)
		return wrapContainer(accentOrange, comps...)
	default:
		allReady := true
		for _, sv := range ps.bg.Servers {
			if sv.Ready != "true" {
				allReady = false
			}
		}
		accent := accentGreen
		if !allReady {
			accent = accentOrange
		}

		for idx, sv := range ps.bg.Servers {
			comps = append(comps, v2Text(formatServer(sv)))
			if idx < len(ps.bg.Servers)-1 {
				div := false
				comps = append(comps, &discordgo.Separator{Divider: &div})
			}
		}

		comps = append(comps, v2Sep(), footer)
		return wrapContainer(accent, comps...)
	}

	comps = append(comps, v2Sep(), footer)
	return wrapContainer(accentGrey, comps...)
}

// wrapContainer puts all components in a single Container with the given accent.
func wrapContainer(accent int, children ...discordgo.MessageComponent) []discordgo.MessageComponent {
	return []discordgo.MessageComponent{
		&discordgo.Container{
			AccentColor: &accent,
			Components:  children,
		},
	}
}

// ── game server type & parser ─────────────────────────────────────────────────

type gameServer struct {
	Map     string
	Phase   string
	Ready   string
	Players string
	Age     string
}

type bgStatus struct {
	Status  string // Healthy, Stopped, etc.
	Uptime  string
	Servers []gameServer
}

// fetchParsedServers runs bg-status and parses the output.
func fetchParsedServers(client *api.Client) (*bgStatus, error) {
	var buf strings.Builder
	_, err := client.Exec(api.ExecRequest{Cmd: "bg-status"}, func(line string) {
		buf.WriteString(line)
	})
	if err != nil {
		return nil, err
	}
	return parseBGStatus(buf.String()), nil
}

// parseBGStatus parses full battlegroup status output into a bgStatus struct.
//
// Battlegroup Info table columns: Status  Database  Gateway  Director  Uptime
// Game Servers table columns:     Map  Phase  Ready  Players  Age
func parseBGStatus(raw string) *bgStatus {
	result := &bgStatus{}
	lines := strings.Split(raw, "\n")

	const (
		sNone       = iota
		sBGInfo     // inside Battlegroup Info section
		sBGData     // past BG info header/separator, reading data row
		sGameHdr    // inside Game Servers section, before separator
		sGameData   // past Game Servers separator, reading rows
	)
	state := sNone

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		switch state {
		case sNone:
			switch {
			case strings.EqualFold(trimmed, "battlegroup info"):
				state = sBGInfo
			case strings.EqualFold(trimmed, "game servers"):
				state = sGameHdr
			}

		case sBGInfo:
			if strings.HasPrefix(trimmed, "---") {
				state = sBGData
			}

		case sBGData:
			if trimmed == "" {
				state = sNone
				continue
			}
			fields := strings.Fields(trimmed)
			if len(fields) >= 1 {
				result.Status = fields[0]
			}
			if len(fields) >= 5 {
				result.Uptime = fields[4]
			}
			state = sNone

		case sGameHdr:
			if strings.HasPrefix(trimmed, "---") {
				state = sGameData
			}

		case sGameData:
			if trimmed == "" {
				state = sNone
				continue
			}
			// "No resources found in ..." — no servers running.
			if strings.HasPrefix(strings.ToLower(trimmed), "no resources") {
				continue
			}
			fields := strings.Fields(trimmed)
			if len(fields) < 4 {
				continue
			}
			sv := gameServer{
				Map:     fields[0],
				Phase:   fields[1],
				Ready:   fields[2],
				Players: fields[3],
			}
			if len(fields) >= 5 {
				sv.Age = fields[4]
			}
			result.Servers = append(result.Servers, sv)
		}
	}
	return result
}

// formatServer renders a single game server as a markdown string.
func formatServer(sv gameServer) string {
	mapName := strings.ReplaceAll(sv.Map, "_", " ")

	phaseEmoji := phaseEmoji(sv.Phase)

	readyStr := ""
	if sv.Ready != "true" {
		readyStr = "  ⚠️ *not ready*"
	}

	age := ""
	if sv.Age != "" {
		age = "  ·  ⏱️ " + sv.Age
	}

	return phaseEmoji + " **" + mapName + "**" + readyStr + "\n" +
		"> Phase: `" + sv.Phase + "`  ·  👥 " + sv.Players + " players" + age
}

func phaseEmoji(phase string) string {
	switch strings.ToLower(phase) {
	case "running":
		return "🟢"
	case "starting", "creating":
		return "🔄"
	case "stopping", "terminating":
		return "🔴"
	default:
		return "⚪"
	}
}

// bgStatusLine formats the battlegroup-level status for display when no game servers are active.
func bgStatusLine(status string) string {
	switch strings.ToLower(status) {
	case "stopped":
		return "🟠 **Battlegroup stopped** — no game servers running"
	case "healthy":
		return "🟢 **Battlegroup healthy** — no game servers found"
	case "":
		return "*(no game servers)*"
	default:
		return "⚪ Battlegroup: **" + status + "** — no game servers running"
	}
}

func boolPtr(b bool) *bool { return &b }
