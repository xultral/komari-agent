package cmd

import (
	"os"
	"path/filepath"
	"testing"

	pkg_flags "github.com/xultral/komari-agent/cmd/flags"
	"github.com/spf13/cobra"
)

func TestLoadConfigPrecedenceAndMonitoringOnlyDefaults(t *testing.T) {
	t.Setenv("AGENT_ENDPOINT", "https://env.example")
	t.Setenv("AGENT_DISABLE_WEB_SSH", "false")
	t.Setenv("AGENT_DISABLE_AUTO_UPDATE", "false")

	configPath := filepath.Join(t.TempDir(), "agent.json")
	configBody := `{
		"endpoint": "https://config.example",
		"disable_web_ssh": false,
		"disable_auto_update": false,
		"protocol_version": 1
	}`
	if err := os.WriteFile(configPath, []byte(configBody), 0o600); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cmd := newTestRootCommand()
	args := []string{
		"--config", configPath,
		"--endpoint", "https://flag.example",
		"--protocol-version", "2",
	}
	if err := cmd.ParseFlags(args); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := loadConfig(cmd); err != nil {
		t.Fatalf("loadConfig returned error: %v", err)
	}

	if flags.Endpoint != "https://flag.example" {
		t.Fatalf("expected CLI endpoint override, got %q", flags.Endpoint)
	}
	if flags.ProtocolVersion != 2 {
		t.Fatalf("expected CLI protocol version override, got %d", flags.ProtocolVersion)
	}
	if !flags.DisableWebSsh {
		t.Fatal("expected remote control to remain disabled in monitoring-only mode")
	}
	if !flags.DisableAutoUpdate {
		t.Fatal("expected auto update to remain disabled in monitoring-only mode")
	}
	if flags.ShowWarning {
		t.Fatal("expected show-warning to be forced off in monitoring-only mode")
	}
}

func TestLoadConfigUsesEnvWhenNoCLIOverride(t *testing.T) {
	t.Setenv("AGENT_ENDPOINT", "https://env.example")
	t.Setenv("AGENT_CONFIG_FILE", "")

	cmd := newTestRootCommand()
	if err := loadConfig(cmd); err != nil {
		t.Fatalf("loadConfig returned error: %v", err)
	}
	if flags.Endpoint != "https://env.example" {
		t.Fatalf("expected env endpoint, got %q", flags.Endpoint)
	}
}

func newTestRootCommand() *cobra.Command {
	*flags = pkg_flags.Config{}
	cmd := &cobra.Command{Use: "komari-agent-test"}
	registerPersistentFlags(cmd)
	return cmd
}
