package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestAgentConfigUnmarshalEnv(t *testing.T) {
	var cfg Config
	data := []byte(`{
		"agents": {
			"claude": {
				"type": "cli",
				"command": "claude",
				"env": {
					"ANTHROPIC_API_KEY": "test-key",
					"EMPTY": ""
				}
			}
		}
	}`)

	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("unmarshal config: %v", err)
	}

	ag, ok := cfg.Agents["claude"]
	if !ok {
		t.Fatalf("expected claude agent config")
	}
	if got := ag.Env["ANTHROPIC_API_KEY"]; got != "test-key" {
		t.Fatalf("ANTHROPIC_API_KEY = %q, want %q", got, "test-key")
	}
	if got, ok := ag.Env["EMPTY"]; !ok || got != "" {
		t.Fatalf("EMPTY = %q, present=%v; want empty string present", got, ok)
	}
}

func TestAgentConfigMarshalEnvRoundTrip(t *testing.T) {
	cfg := Config{
		Agents: map[string]AgentConfig{
			"claude": {
				Type:    "cli",
				Command: "claude",
				Env: map[string]string{
					"ANTHROPIC_API_KEY": "test-key",
					"EMPTY":             "",
				},
			},
		},
	}

	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}

	var decoded Config
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("round-trip unmarshal: %v", err)
	}

	got := decoded.Agents["claude"].Env
	if got["ANTHROPIC_API_KEY"] != "test-key" || got["EMPTY"] != "" {
		t.Fatalf("round-trip env = %#v", got)
	}
}

func TestAgentConfigWithoutEnvStillLoads(t *testing.T) {
	var cfg Config
	data := []byte(`{
		"agents": {
			"claude": {
				"type": "cli",
				"command": "claude"
			}
		}
	}`)

	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("unmarshal config without env: %v", err)
	}

	if cfg.Agents["claude"].Env != nil {
		t.Fatalf("Env = %#v, want nil", cfg.Agents["claude"].Env)
	}
}

func TestDefaultConfigInitializesAgentsMap(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Agents == nil {
		t.Fatal("DefaultConfig() Agents = nil, want initialized map")
	}
}

func TestLoadEnvOverridesTopLevelOnly(t *testing.T) {
	t.Setenv("WECLAW_DEFAULT_AGENT", "codex")
	t.Setenv("WECLAW_API_ADDR", "127.0.0.1:18011")
	t.Setenv("WECLAW_REPLY_ENDPOINT", "http://127.0.0.1:8000/chat")

	cfg := DefaultConfig()
	cfg.Agents["claude"] = AgentConfig{
		Type: "cli",
		Env: map[string]string{
			"KEEP": "value",
		},
	}

	loadEnv(cfg)

	if cfg.DefaultAgent != "codex" {
		t.Fatalf("DefaultAgent = %q, want %q", cfg.DefaultAgent, "codex")
	}
	if cfg.APIAddr != "127.0.0.1:18011" {
		t.Fatalf("APIAddr = %q, want %q", cfg.APIAddr, "127.0.0.1:18011")
	}
	if cfg.ReplyEndpoint != "http://127.0.0.1:8000/chat" {
		t.Fatalf("ReplyEndpoint = %q, want %q", cfg.ReplyEndpoint, "http://127.0.0.1:8000/chat")
	}
	if got := cfg.Agents["claude"].Env["KEEP"]; got != "value" {
		t.Fatalf("agent env = %q, want preserved value", got)
	}
}

func TestLoadDotEnvSetsMissingVars(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	if err := os.WriteFile(path, []byte("WECLAW_REPLY_ENDPOINT=http://127.0.0.1:8000/chat\n"), 0o600); err != nil {
		t.Fatalf("write .env: %v", err)
	}

	t.Setenv("WECLAW_REPLY_ENDPOINT", "")
	if err := os.Unsetenv("WECLAW_REPLY_ENDPOINT"); err != nil {
		t.Fatalf("unset env: %v", err)
	}

	if err := LoadDotEnv(path); err != nil {
		t.Fatalf("LoadDotEnv() error = %v", err)
	}

	if got := os.Getenv("WECLAW_REPLY_ENDPOINT"); got != "http://127.0.0.1:8000/chat" {
		t.Fatalf("WECLAW_REPLY_ENDPOINT = %q, want %q", got, "http://127.0.0.1:8000/chat")
	}
}

func TestLoadDotEnvDoesNotOverrideExistingVars(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	if err := os.WriteFile(path, []byte("WECLAW_REPLY_ENDPOINT=http://127.0.0.1:8000/chat\n"), 0o600); err != nil {
		t.Fatalf("write .env: %v", err)
	}

	t.Setenv("WECLAW_REPLY_ENDPOINT", "http://127.0.0.1:9000/chat")

	if err := LoadDotEnv(path); err != nil {
		t.Fatalf("LoadDotEnv() error = %v", err)
	}

	if got := os.Getenv("WECLAW_REPLY_ENDPOINT"); got != "http://127.0.0.1:9000/chat" {
		t.Fatalf("WECLAW_REPLY_ENDPOINT = %q, want %q", got, "http://127.0.0.1:9000/chat")
	}
}
