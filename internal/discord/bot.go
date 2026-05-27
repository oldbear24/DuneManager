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
	dg       *discordgo.Session
	client   *api.Client
	guildID  string
	chanID   string
	roleID   string
	commands []*discordgo.ApplicationCommand
}

// New creates a Bot but does not connect yet.
// guildID: register commands for this guild only (instant); empty = global (up to 1h delay).
// channelID: restrict commands to this channel; empty = allow everywhere.
// roleID: restrict bot use to members who have this role; empty = allow all members.
func New(token, guildID, channelID, roleID string) (*Bot, error) {
	dg, err := discordgo.New("Bot " + token)
	if err != nil {
		return nil, fmt.Errorf("discord session: %w", err)
	}
	return &Bot{
		dg:      dg,
		client:  api.NewClient(),
		guildID: guildID,
		chanID:  channelID,
		roleID:  roleID,
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
	return nil
}

// Stop removes registered commands and closes the session.
func (b *Bot) Stop() {
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

// ── command handlers ──────────────────────────────────────────────────────────

func (b *Bot) handleStatus(s *discordgo.Session, i *discordgo.InteractionCreate) {
	_ = s.InteractionRespond(i.Interaction, deferredResponse())

	status, err := b.client.GetStatus()
	if err != nil {
		_, _ = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
			Content: fmt.Sprintf("❌ Service offline: %v", err),
		})
		return
	}

	vmState := status.VMState
	if status.Running {
		vmState = "Running ✅"
	} else if vmState == "Off" {
		vmState = "Off 🟠"
	} else if vmState == "missing" {
		vmState = "Not installed ❌"
	}

	ip := status.IP
	if ip == "" {
		ip = "—"
	}
	busy := "No"
	if status.Busy {
		busy = "Yes ⏳"
	}

	_, _ = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
		Embeds: []*discordgo.MessageEmbed{
			{
				Title: "🏜️ Dune Server Status",
				Color: embedColor(status),
				Fields: []*discordgo.MessageEmbedField{
					{Name: "VM State", Value: vmState, Inline: true},
					{Name: "IP", Value: ip, Inline: true},
					{Name: "Busy", Value: busy, Inline: true},
				},
			},
		},
	})
}

func (b *Bot) handleExec(s *discordgo.Session, i *discordgo.InteractionCreate, req api.ExecRequest, title string) {
	_ = s.InteractionRespond(i.Interaction, deferredResponse())

	// Send initial placeholder; keep the message ID for live edits.
	initialContent := fmt.Sprintf("⏳ **%s** running…", title)
	msg, err := s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
		Content: initialContent,
	})
	msgID := ""
	if err == nil {
		msgID = msg.ID
	}

	var buf strings.Builder
	lastEdit := time.Now()

	onLine := func(line string) {
		buf.WriteString(line)
		// Rate-limit live edits to once every 2 seconds.
		if msgID == "" || time.Since(lastEdit) < 2*time.Second {
			return
		}
		lastEdit = time.Now()
		content := fmtOutput(title+" (running…)", buf.String())
		_, _ = s.FollowupMessageEdit(i.Interaction, msgID, &discordgo.WebhookEdit{
			Content: &content,
		})
	}

	_, execErr := b.client.Exec(req, onLine)

	var finalContent string
	if execErr != nil {
		finalContent = fmt.Sprintf("❌ **%s** failed\n```\n%s\nError: %v\n```",
			title, truncateTail(buf.String(), maxMsgLen-200), execErr)
	} else {
		finalContent = fmtOutput("✅ "+title+" done", buf.String())
	}

	if msgID != "" {
		_, _ = s.FollowupMessageEdit(i.Interaction, msgID, &discordgo.WebhookEdit{
			Content: &finalContent,
		})
	} else {
		_, _ = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
			Content: finalContent,
		})
	}
}

func (b *Bot) handleKill(s *discordgo.Session, i *discordgo.InteractionCreate) {
	err := b.client.Kill()
	content := "⏹ Kill signal sent."
	if err != nil {
		content = fmt.Sprintf("❌ Kill failed: %v", err)
	}
	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{Content: content},
	})
}

// ── helpers ────────────────────────────────────────────────────────────────────

func deferredResponse() *discordgo.InteractionResponse {
	return &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
	}
}

func fmtOutput(title, output string) string {
	return fmt.Sprintf("**%s**\n```\n%s\n```", title, truncateTail(output, maxMsgLen))
}

// truncateTail keeps the last `max` bytes of s, prepending a marker if cut.
func truncateTail(s string, max int) string {
	s = strings.TrimRight(s, "\n")
	if len(s) <= max {
		return s
	}
	cut := s[len(s)-max:]
	// Align to a newline so we don't split mid-line.
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

func embedColor(s *api.StatusResponse) int {
	switch {
	case s.Running:
		return 0x00C850
	case s.VMState == "Off":
		return 0xC87800
	case s.VMState == "missing":
		return 0xC82828
	default:
		return 0x787878
	}
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
