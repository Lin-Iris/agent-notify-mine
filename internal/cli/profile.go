package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/hellolib/agent-notify/internal/config"
	"github.com/hellolib/agent-notify/internal/feishucli"
	"github.com/hellolib/agent-notify/internal/notify"
	"github.com/spf13/cobra"
)

var profileFeishuReinitialize = feishucli.Reinitialize
var profileFeishuEnsureReady = feishucli.EnsureReady
var profileFeishuResolveOwnerOpenID = notify.ResolveFeishuAppCreatorOpenID

func newProfileCmd(streams Streams) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "profile",
		Short: "Manage broker profiles",
	}
	cmd.AddCommand(
		newProfileCreateCmd(streams),
		newProfileListCmd(streams),
		newProfileUseCmd(streams),
		newProfileFeishuCmd(streams),
		newProfileRemoveCmd(streams),
	)
	return cmd
}

func newProfileCreateCmd(streams Streams) *cobra.Command {
	var agent, workspace, permission string
	cmd := &cobra.Command{
		Use:   "create <name>",
		Short: "Create or update a profile",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, path, err := loadDefaultConfig()
			if err != nil {
				return err
			}
			name := args[0]
			if agent == "" {
				agent = "claude"
			}
			if permission == "" {
				permission = "workspace-write"
			}
			if workspace != "" {
				abs, err := filepath.Abs(workspace)
				if err != nil {
					return err
				}
				if info, err := os.Stat(abs); err != nil || !info.IsDir() {
					return fmt.Errorf("workspace is not a directory: %s", workspace)
				}
				workspace = abs
			}
			if cfg.Profiles == nil {
				cfg.Profiles = config.ProfilesConfig{}
			}
			cfg.Profiles[name] = config.ProfileConfig{
				Agent:          agent,
				Workspace:      workspace,
				Enabled:        false,
				PermissionMode: permission,
				Workspaces:     map[string]string{},
			}
			if cfg.Broker.ActiveProfile == "" {
				cfg.Broker.ActiveProfile = name
			}
			if err := config.Save(path, cfg); err != nil {
				return err
			}
			fmt.Fprintf(streams.Stdout, "profile saved: %s agent=%s workspace=%s permission=%s\n", name, agent, workspace, permission)
			return appendAudit("profile create name=" + name)
		},
	}
	cmd.Flags().StringVar(&agent, "agent", "claude", "claude or codex")
	cmd.Flags().StringVar(&workspace, "workspace", "", "default workspace")
	cmd.Flags().StringVar(&permission, "permission", "workspace-write", "read-only or workspace-write")
	return cmd
}

func newProfileListCmd(streams Streams) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List profiles",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _, err := loadDefaultConfig()
			if err != nil {
				return err
			}
			for name, profile := range cfg.Profiles {
				active := ""
				if name == cfg.Broker.ActiveProfile {
					active = "*"
				}
				feishu := "unbound"
				if profile.Feishu.AppID != "" && profile.Feishu.AppSecret != "" && profile.Feishu.OwnerOpenID != "" {
					feishu = "bound"
					if profile.Feishu.ChatID != "" {
						feishu += " chat_id=" + profile.Feishu.ChatID
					}
				}
				fmt.Fprintf(streams.Stdout, "%s%s agent=%s enabled=%v workspace=%s permission=%s feishu=%s\n",
					active, name, profile.Agent, profile.Enabled, profile.Workspace, profile.PermissionMode, feishu)
			}
			return nil
		},
	}
}

func newProfileFeishuCmd(streams Streams) *cobra.Command {
	var appID, appSecret, ownerOpenID, chatID string
	cmd := &cobra.Command{
		Use:   "feishu <profile>",
		Short: "Bind a profile to a Feishu bot",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, path, err := loadDefaultConfig()
			if err != nil {
				return err
			}
			name := args[0]
			profile, ok := cfg.Profiles[name]
			if !ok {
				return fmt.Errorf("profile %s not found", name)
			}
			if appID != "" {
				profile.Feishu.AppID = appID
			}
			if appSecret != "" {
				profile.Feishu.AppSecret = appSecret
			}
			if ownerOpenID != "" {
				profile.Feishu.OwnerOpenID = ownerOpenID
			}
			if cmd.Flags().Changed("chat-id") {
				profile.Feishu.ChatID = chatID
			}
			if profile.Feishu.AppID == "" || profile.Feishu.AppSecret == "" || profile.Feishu.OwnerOpenID == "" {
				return fmt.Errorf("profile %s feishu requires --app-id, --app-secret, and --owner-open-id", name)
			}
			cfg.Profiles[name] = profile
			if err := config.Save(path, cfg); err != nil {
				return err
			}
			target := "owner_open_id"
			if profile.Feishu.ChatID != "" {
				target = "chat_id=" + profile.Feishu.ChatID
			}
			fmt.Fprintf(streams.Stdout, "profile feishu saved: %s app_id=%s target=%s\n", name, profile.Feishu.AppID, target)
			return appendAudit("profile feishu name=" + name)
		},
	}
	cmd.Flags().StringVar(&appID, "app-id", "", "Feishu app_id for this profile")
	cmd.Flags().StringVar(&appSecret, "app-secret", "", "Feishu app_secret for this profile")
	cmd.Flags().StringVar(&ownerOpenID, "owner-open-id", "", "Feishu owner open_id for this bot")
	cmd.Flags().StringVar(&chatID, "chat-id", "", "optional Feishu chat_id target")
	cmd.AddCommand(newProfileFeishuSetupCmd(streams))
	return cmd
}

func newProfileFeishuSetupCmd(streams Streams) *cobra.Command {
	var chatID string
	cmd := &cobra.Command{
		Use:   "setup <profile>",
		Short: "Bind a profile to a Feishu bot by QR login",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, path, err := loadDefaultConfig()
			if err != nil {
				return err
			}
			name := args[0]
			profile, ok := cfg.Profiles[name]
			if !ok {
				return fmt.Errorf("profile %s not found", name)
			}
			fmt.Fprintf(streams.Stdout, "请按提示扫码绑定 %s 的飞书机器人...\n", name)
			feishuCfg, err := profileFeishuReinitialize(cmd.Context())
			if err != nil {
				return err
			}
			if feishuCfg.AppID == "" || feishuCfg.AppSecret == "" {
				return fmt.Errorf("feishu QR setup did not return complete bot config")
			}
			if feishuCfg.UserOpenID == "" {
				ownerOpenID, err := profileFeishuResolveOwnerOpenID(cmd.Context(), feishuCfg.AppID, feishuCfg.AppSecret)
				if err != nil {
					return fmt.Errorf("feishu QR setup missing owner open_id and fallback lookup failed: %w", err)
				}
				feishuCfg.UserOpenID = ownerOpenID
			}
			oldChatID := profile.Feishu.ChatID
			profile.Feishu = config.ProfileFeishuConfig{
				AppID:       feishuCfg.AppID,
				AppSecret:   feishuCfg.AppSecret,
				OwnerOpenID: feishuCfg.UserOpenID,
				ChatID:      oldChatID,
			}
			if cmd.Flags().Changed("chat-id") {
				profile.Feishu.ChatID = chatID
			}
			cfg.Profiles[name] = profile
			if err := config.Save(path, cfg); err != nil {
				return err
			}
			target := "owner_open_id"
			if profile.Feishu.ChatID != "" {
				target = "chat_id=" + profile.Feishu.ChatID
			}
			user := feishuCfg.UserName
			if user == "" {
				user = feishuCfg.UserOpenID
			}
			fmt.Fprintf(streams.Stdout, "profile feishu setup saved: %s app_id=%s owner=%s target=%s\n", name, profile.Feishu.AppID, user, target)
			return appendAudit("profile feishu setup name=" + name)
		},
	}
	cmd.Flags().StringVar(&chatID, "chat-id", "", "optional Feishu chat_id target")
	return cmd
}

func resetProfileFeishuReinitializeForTest(fn func(context.Context) (feishucli.Config, error)) func() {
	old := profileFeishuReinitialize
	profileFeishuReinitialize = fn
	return func() {
		profileFeishuReinitialize = old
	}
}

func resetProfileFeishuEnsureReadyForTest(fn func(context.Context) (feishucli.Config, error)) func() {
	old := profileFeishuEnsureReady
	profileFeishuEnsureReady = fn
	return func() {
		profileFeishuEnsureReady = old
	}
}

func newProfileUseCmd(streams Streams) *cobra.Command {
	return &cobra.Command{
		Use:   "use <name>",
		Short: "Set active profile",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, path, err := loadDefaultConfig()
			if err != nil {
				return err
			}
			if _, ok := cfg.Profiles[args[0]]; !ok {
				return fmt.Errorf("profile %s not found", args[0])
			}
			cfg.Broker.ActiveProfile = args[0]
			if err := config.Save(path, cfg); err != nil {
				return err
			}
			fmt.Fprintf(streams.Stdout, "active profile: %s\n", args[0])
			return appendAudit("profile use name=" + args[0])
		},
	}
}

func newProfileRemoveCmd(streams Streams) *cobra.Command {
	return &cobra.Command{
		Use:   "remove <name>",
		Short: "Remove a profile",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, path, err := loadDefaultConfig()
			if err != nil {
				return err
			}
			delete(cfg.Profiles, args[0])
			if cfg.Broker.ActiveProfile == args[0] {
				cfg.Broker.ActiveProfile = ""
				for name := range cfg.Profiles {
					cfg.Broker.ActiveProfile = name
					break
				}
			}
			if err := config.Save(path, cfg); err != nil {
				return err
			}
			fmt.Fprintf(streams.Stdout, "profile removed: %s\n", args[0])
			return appendAudit("profile remove name=" + args[0])
		},
	}
}

func newPSCmd(streams Streams) *cobra.Command {
	var profile string
	cmd := &cobra.Command{
		Use:   "ps",
		Short: "List controlled agent processes",
		RunE: func(cmd *cobra.Command, args []string) error {
			items, err := listProcesses(profile)
			if err != nil {
				return err
			}
			for _, item := range items {
				fmt.Fprintf(streams.Stdout, "%s pid=%d profile=%s agent=%s status=%s workspace=%s log=%s\n",
					item.ID, item.PID, item.Profile, item.Agent, item.Status, item.Workspace, item.LogPath)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&profile, "profile", "", "profile name")
	return cmd
}

func newKillCmd(streams Streams) *cobra.Command {
	return &cobra.Command{
		Use:   "kill <id|pid>",
		Short: "Stop a controlled agent process",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			reg, _, err := processRegistry()
			if err != nil {
				return err
			}
			if err := reg.Kill(args[0]); err != nil {
				return err
			}
			fmt.Fprintf(streams.Stdout, "kill requested: %s\n", args[0])
			return appendAudit("process kill id=" + args[0])
		},
	}
}
