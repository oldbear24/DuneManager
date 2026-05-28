package discord

import (
	"fmt"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"

	"github.com/oldbear24/DuneManager/internal/api"
	"github.com/oldbear24/DuneManager/internal/logging"
)

const maxMsgLen = 1900 // Discord cap is 2000; leave margin for formatting

// Bot wraps a discordgo session and routes slash commands to the HTTP service.
type Bot struct {
	dg           *discordgo.Session
	client       *api.Client
	pollerClient *api.Client
	guildID      string
	chanID       string // command restriction channel
	roleID       string
	statusChanID string // channel for the live status embed
	commands     []*discordgo.ApplicationCommand
	pollerStop   chan struct{}
	statusMsgID  string
}

// New creates a Bot but does not connect yet.
// guildID: register commands for this guild only (instant); empty = global (up to 1h delay).
// channelID: restrict commands to this channel; empty = allow everywhere.
// roleID: restrict bot use to members who have this role; empty = allow all members.
// statusChannelID: post/edit the live status embed in this channel; empty = no embed.
func New(token, guildID, channelID, roleID, statusChannelID string) (*Bot, error) {
	dg, err := discordgo.New("Bot " + token)
	if err != nil {
		return nil, fmt.Errorf("discord session: %w", err)
	}
	return &Bot{
		dg:           dg,
		client:       api.NewClient(),
		pollerClient: api.NewClientWithTimeout(45 * time.Second),
		guildID:      guildID,
		chanID:       channelID,
		roleID:       roleID,
		statusChanID: statusChannelID,
	}, nil
}

// commandDefs describes the /dune slash command tree.
var commandDefs = []*discordgo.ApplicationCommand{
	{
		Name:        "dune",
		Description: "Dune Awakening server manager",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Name:        "status",
				Description: "Show current VM and service status",
			},
			{
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Name:        "vm-start",
				Description: "Start the Hyper-V VM",
			},
			{
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Name:        "vm-stop",
				Description: "Stop the Hyper-V VM",
			},
			{
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Name:        "ssh-rotate",
				Description: "Rotate the VM SSH key",
			},
			{
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Name:        "bg",
				Description: "Battlegroup management",
				Options: []*discordgo.ApplicationCommandOption{
					{
						Type:        discordgo.ApplicationCommandOptionString,
						Name:        "action",
						Description: "Action to perform",
						Required:    true,
						Choices: []*discordgo.ApplicationCommandOptionChoice{
							{Name: "status", Value: "bg-status"},
							{Name: "start", Value: "bg-start"},
							{Name: "stop", Value: "bg-stop"},
							{Name: "restart", Value: "bg-restart"},
							{Name: "update", Value: "bg-update"},
							{Name: "backup", Value: "bg-backup"},
							{Name: "swap", Value: "bg-swap"},
						},
					},
				},
			},
			{
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Name:        "kill",
				Description: "Kill the currently running command",
			},
		},
	},
}

// Start opens the WebSocket connection and registers slash commands.
func (b *Bot) Start() error {
	b.dg.AddHandler(b.onInteraction)
	b.dg.Identify.Intents = discordgo.IntentsGuilds

	if err := b.dg.Open(); err != nil {
		return fmt.Errorf("discord open: %w", err)
	}

	for _, def := range commandDefs {
		cmd, err := b.dg.ApplicationCommandCreate(b.dg.State.User.ID, b.guildID, def)
		if err != nil {
			return fmt.Errorf("register command %q: %w", def.Name, err)
		}
		b.commands = append(b.commands, cmd)
	}
	if b.statusChanID != "" {
		b.startPoller()
	}
	return nil
}

// Stop removes registered commands, stops the poller, and closes the session.
func (b *Bot) Stop() {
	b.stopPoller()
	appID := b.dg.State.User.ID
	for _, cmd := range b.commands {
		_ = b.dg.ApplicationCommandDelete(appID, b.guildID, cmd.ID)
	}
	_ = b.dg.Close()
}

// ── interaction handler ────────────────────────────────────────────────────────

func (b *Bot) onInteraction(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if i.Type != discordgo.InteractionApplicationCommand {
		return
	}

	// Channel restriction.
	if b.chanID != "" && i.ChannelID != b.chanID {
		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "❌ Dune Manager commands are not allowed in this channel.",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		return
	}
	if !b.memberAllowed(i.Member) {
		userID := "unknown"
		if i.Member != nil && i.Member.User != nil {
			userID = i.Member.User.ID
		}
		logging.Warningf("Discord access denied for user %s: missing required role %s", userID, b.roleID)
		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "❌ You do not have permission to use Dune Manager commands.",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		return
	}

	data := i.ApplicationCommandData()
	if data.Name != "dune" || len(data.Options) == 0 {
		return
	}

	sub := data.Options[0]
	switch sub.Name {
	case "status":
		b.handleStatus(s, i)
	case "vm-start":
		b.handleExec(s, i, api.ExecRequest{Cmd: "vm-start"}, "VM Start")
	case "vm-stop":
		b.handleExec(s, i, api.ExecRequest{Cmd: "vm-stop"}, "VM Stop")
	case "ssh-rotate":
		b.handleExec(s, i, api.ExecRequest{Cmd: "ssh-rotate"}, "SSH Rotate")
	case "bg":
		cmd := optString(sub.Options, "action")
		b.handleExec(s, i, api.ExecRequest{Cmd: cmd}, cmd)
	case "kill":
		b.handleKill(s, i)
	}
}

const maxTextLen = 3800 // conservative limit for a single TextDisplay

const (
	accentGreen  = 0x00C850
	accentOrange = 0xC87800
	accentRed    = 0xC82828
	accentGrey   = 0x787878
)

// ── command handlers ──────────────────────────────────────────────────────────

func (b *Bot) handleStatus(s *discordgo.Session, i *discordgo.InteractionCreate) {
	_ = s.InteractionRespond(i.Interaction, deferredResponse())

	status, err := b.client.GetStatus()
	if err != nil {
		_, _ = s.FollowupMessageCreate(i.Interaction, true, v2Followup(
			v2Container(accentRed,
				v2Text("## 🏜️ Server Status"),
				v2Sep(),
				v2Text("🔴 **Service offline**: "+err.Error()),
			),
		))
		return
	}

	var stateLine string
	accent := accentGrey
	switch {
	case status.Running:
		stateLine = "🟢 **Running**"
		accent = accentGreen
	case status.VMState == "Off":
		stateLine = "🟠 **VM Off**"
		accent = accentOrange
	case status.VMState == "missing":
		stateLine = "❌ **VM not installed**"
		accent = accentRed
	default:
		stateLine = "⚪ " + status.VMState
	}

	busy := ""
	if status.Busy {
		busy = "\n⏳ A command is currently running."
	}

	_, _ = s.FollowupMessageCreate(i.Interaction, true, v2Followup(
		v2Container(accent,
			v2Text("## 🏜️ Server Status"),
			v2Sep(),
			v2Text(stateLine+busy),
		),
	))
}

func (b *Bot) handleExec(s *discordgo.Session, i *discordgo.InteractionCreate, req api.ExecRequest, title string) {
	_ = s.InteractionRespond(i.Interaction, deferredResponse())

	msg, err := s.FollowupMessageCreate(i.Interaction, true, v2Followup(
		v2Container(accentOrange,
			v2Text("## ⏳ "+title),
			v2Sep(),
			v2Text("*Running…*"),
		),
	))
	msgID := ""
	if err == nil {
		msgID = msg.ID
	}

	var buf strings.Builder
	lastEdit := time.Now()

	onLine := func(line string) {
		buf.WriteString(line)
		if msgID == "" || time.Since(lastEdit) < 2*time.Second {
			return
		}
		lastEdit = time.Now()
		comps := []discordgo.MessageComponent{
			v2Container(accentOrange,
				v2Text("## ⏳ "+title),
				v2Sep(),
				v2Text("```\n"+truncateTail(buf.String(), maxTextLen)+"\n```"),
			),
		}
		_, _ = s.FollowupMessageEdit(i.Interaction, msgID, &discordgo.WebhookEdit{
			Components: &comps,
		})
	}

	_, execErr := b.client.Exec(req, onLine)

	var finalComps []discordgo.MessageComponent
	if execErr != nil {
		finalComps = []discordgo.MessageComponent{
			v2Container(accentRed,
				v2Text("## ❌ "+title+" — failed"),
				v2Sep(),
				v2Text("```\n"+truncateTail(buf.String(), maxTextLen-100)+"\n```\n**Error:** "+execErr.Error()),
			),
		}
	} else {
		out := strings.TrimSpace(buf.String())
		body := "*No output.*"
		if out != "" {
			body = "```\n" + truncateTail(out, maxTextLen) + "\n```"
		}
		finalComps = []discordgo.MessageComponent{
			v2Container(accentGreen,
				v2Text("## ✅ "+title),
				v2Sep(),
				v2Text(body),
			),
		}
	}

	if msgID != "" {
		_, _ = s.FollowupMessageEdit(i.Interaction, msgID, &discordgo.WebhookEdit{
			Components: &finalComps,
		})
	} else {
		_, _ = s.FollowupMessageCreate(i.Interaction, true, v2Followup(finalComps...))
	}
}

func (b *Bot) handleKill(s *discordgo.Session, i *discordgo.InteractionCreate) {
	err := b.client.Kill()
	accent := accentGrey
	text := "⏹ **Kill signal sent.**"
	if err != nil {
		accent = accentRed
		text = "❌ Kill failed: " + err.Error()
	}
	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Flags:      discordgo.MessageFlagsIsComponentsV2 | discordgo.MessageFlagsEphemeral,
			Components: []discordgo.MessageComponent{v2Container(accent, v2Text(text))},
		},
	})
}

// ── helpers ────────────────────────────────────────────────────────────────────

func deferredResponse() *discordgo.InteractionResponse {
	return &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Flags: discordgo.MessageFlagsEphemeral,
		},
	}
}

// v2Container wraps components in a Container with an accent colour bar.
func v2Container(accent int, children ...discordgo.MessageComponent) *discordgo.Container {
	return &discordgo.Container{
		AccentColor: &accent,
		Components:  children,
	}
}

// v2Text returns a TextDisplay component.
func v2Text(content string) *discordgo.TextDisplay {
	return &discordgo.TextDisplay{Content: content}
}

// v2Sep returns a Separator with a visible divider line.
func v2Sep() *discordgo.Separator {
	return &discordgo.Separator{}
}

// v2Followup builds a WebhookParams ready for FollowupMessageCreate with Components V2 (ephemeral).
func v2Followup(comps ...discordgo.MessageComponent) *discordgo.WebhookParams {
	return &discordgo.WebhookParams{
		Flags:      discordgo.MessageFlagsIsComponentsV2 | discordgo.MessageFlagsEphemeral,
		Components: comps,
	}
}

// truncateTail keeps the last `max` bytes of s, prepending a marker if cut.
func truncateTail(s string, max int) string {
	s = strings.TrimRight(s, "\n")
	if len(s) <= max {
		return s
	}
	cut := s[len(s)-max:]
	if idx := strings.Index(cut, "\n"); idx >= 0 {
		cut = cut[idx+1:]
	}
	return "[…output truncated…]\n" + cut
}

func optString(opts []*discordgo.ApplicationCommandInteractionDataOption, name string) string {
	for _, o := range opts {
		if o.Name == name {
			return o.StringValue()
		}
	}
	return ""
}

func (b *Bot) memberAllowed(member *discordgo.Member) bool {
	if b.roleID == "" {
		return true
	}
	if member == nil {
		return false
	}
	for _, roleID := range member.Roles {
		if roleID == b.roleID {
			return true
		}
	}
	return false
}
