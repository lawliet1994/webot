package messaging

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/fastclaw-ai/weclaw/agent"
	"github.com/fastclaw-ai/weclaw/ilink"
)

func newTestHandler() *Handler {
	return &Handler{agents: make(map[string]agent.Agent)}
}

type stubAgent struct {
	reply string
	err   error
	chat  func(ctx context.Context, conversationID string, message string) (string, error)
}

func (s stubAgent) Chat(ctx context.Context, conversationID string, message string) (string, error) {
	if s.chat != nil {
		return s.chat(ctx, conversationID, message)
	}
	return s.reply, s.err
}

func (s stubAgent) ResetSession(ctx context.Context, conversationID string) (string, error) {
	return "", nil
}

func (s stubAgent) Info() agent.AgentInfo {
	return agent.AgentInfo{Name: "stub", Type: "test", Command: "stub"}
}

func (s stubAgent) SetCwd(cwd string) {}

func TestChatWithAgent_UsesReplyEndpoint(t *testing.T) {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/chat" {
			t.Fatalf("path = %s, want /chat", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"reply":"from-local-server"}`))
	}))
	defer srv.Close()

	h := newTestHandler()
	h.replyEndpoint = srv.URL + "/chat"

	reply, err := h.chatWithAgent(context.Background(), stubAgent{
		chat: func(ctx context.Context, conversationID string, message string) (string, error) {
			t.Fatal("agent Chat should not be called when reply endpoint is configured")
			return "", nil
		},
	}, "user-1", "hello")
	if err != nil {
		t.Fatalf("chatWithAgent error = %v", err)
	}
	if reply != "from-local-server" {
		t.Fatalf("reply = %q, want %q", reply, "from-local-server")
	}
}

func TestChatWithAgent_FallsBackToAgent(t *testing.T) {
	h := newTestHandler()

	reply, err := h.chatWithAgent(context.Background(), stubAgent{reply: "from-agent"}, "user-1", "hello")
	if err != nil {
		t.Fatalf("chatWithAgent error = %v", err)
	}
	if reply != "from-agent" {
		t.Fatalf("reply = %q, want %q", reply, "from-agent")
	}
}

func TestFetchReply_IncludesFilePayload(t *testing.T) {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var got localReplyRequest
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if got.UserID != "user-1" {
			t.Fatalf("user_id = %q, want %q", got.UserID, "user-1")
		}
		if got.Message != "" {
			t.Fatalf("message = %q, want empty string", got.Message)
		}
		if got.File == nil {
			t.Fatal("file payload is nil")
		}
		if got.File.Name != "report.xlsx" {
			t.Fatalf("file.name = %q, want %q", got.File.Name, "report.xlsx")
		}
		if got.File.Size != "1234" {
			t.Fatalf("file.size = %q, want %q", got.File.Size, "1234")
		}
		if got.File.EncryptQueryParam != "enc-param" {
			t.Fatalf("file.encrypt_query_param = %q, want %q", got.File.EncryptQueryParam, "enc-param")
		}
		if got.File.AESKey != "aes-key" {
			t.Fatalf("file.aes_key = %q, want %q", got.File.AESKey, "aes-key")
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"reply":"file-received"}`))
	}))
	defer srv.Close()

	h := newTestHandler()
	reply, err := h.fetchReply(context.Background(), nil, srv.URL, localReplyRequest{
		UserID: "user-1",
		File: &localReplyFile{
			Name:              "report.xlsx",
			Size:              "1234",
			EncryptQueryParam: "enc-param",
			AESKey:            "aes-key",
		},
	})
	if err != nil {
		t.Fatalf("fetchReply error = %v", err)
	}
	if reply != "file-received" {
		t.Fatalf("reply = %q, want %q", reply, "file-received")
	}
}

func TestExtractFile(t *testing.T) {
	msg := ilink.WeixinMessage{
		ItemList: []ilink.MessageItem{
			{
				Type: ilink.ItemTypeFile,
				FileItem: &ilink.FileItem{
					FileName: "report.xlsx",
				},
			},
		},
	}

	file := extractFile(msg)
	if file == nil {
		t.Fatal("extractFile() = nil, want file item")
	}
	if file.FileName != "report.xlsx" {
		t.Fatalf("file.FileName = %q, want %q", file.FileName, "report.xlsx")
	}
}

func TestParseCommand_NoPrefix(t *testing.T) {
	h := newTestHandler()
	names, msg := h.parseCommand("hello world")
	if len(names) != 0 {
		t.Errorf("expected nil names, got %v", names)
	}
	if msg != "hello world" {
		t.Errorf("expected full text, got %q", msg)
	}
}

func TestParseCommand_SlashWithAgent(t *testing.T) {
	h := newTestHandler()
	names, msg := h.parseCommand("/claude explain this code")
	if len(names) != 1 || names[0] != "claude" {
		t.Errorf("expected [claude], got %v", names)
	}
	if msg != "explain this code" {
		t.Errorf("expected 'explain this code', got %q", msg)
	}
}

func TestParseCommand_AtPrefix(t *testing.T) {
	h := newTestHandler()
	names, msg := h.parseCommand("@claude explain this code")
	if len(names) != 1 || names[0] != "claude" {
		t.Errorf("expected [claude], got %v", names)
	}
	if msg != "explain this code" {
		t.Errorf("expected 'explain this code', got %q", msg)
	}
}

func TestParseCommand_MultiAgent(t *testing.T) {
	h := newTestHandler()
	names, msg := h.parseCommand("@cc @cx hello")
	if len(names) != 2 || names[0] != "claude" || names[1] != "codex" {
		t.Errorf("expected [claude codex], got %v", names)
	}
	if msg != "hello" {
		t.Errorf("expected 'hello', got %q", msg)
	}
}

func TestParseCommand_MultiAgentDedup(t *testing.T) {
	h := newTestHandler()
	names, msg := h.parseCommand("@cc @cc hello")
	if len(names) != 1 || names[0] != "claude" {
		t.Errorf("expected [claude] (deduped), got %v", names)
	}
	if msg != "hello" {
		t.Errorf("expected 'hello', got %q", msg)
	}
}

func TestParseCommand_SwitchOnly(t *testing.T) {
	h := newTestHandler()
	names, msg := h.parseCommand("/claude")
	if len(names) != 1 || names[0] != "claude" {
		t.Errorf("expected [claude], got %v", names)
	}
	if msg != "" {
		t.Errorf("expected empty message, got %q", msg)
	}
}

func TestParseCommand_Alias(t *testing.T) {
	h := newTestHandler()
	names, msg := h.parseCommand("/cc write a function")
	if len(names) != 1 || names[0] != "claude" {
		t.Errorf("expected [claude] from /cc alias, got %v", names)
	}
	if msg != "write a function" {
		t.Errorf("expected 'write a function', got %q", msg)
	}
}

func TestParseCommand_CustomAlias(t *testing.T) {
	h := newTestHandler()
	h.customAliases = map[string]string{"ai": "claude", "c": "claude"}
	names, msg := h.parseCommand("/ai hello")
	if len(names) != 1 || names[0] != "claude" {
		t.Errorf("expected [claude] from custom alias, got %v", names)
	}
	if msg != "hello" {
		t.Errorf("expected 'hello', got %q", msg)
	}
}

func TestResolveAlias(t *testing.T) {
	h := newTestHandler()
	tests := map[string]string{
		"cc":  "claude",
		"cx":  "codex",
		"oc":  "openclaw",
		"cs":  "cursor",
		"km":  "kimi",
		"gm":  "gemini",
		"ocd": "opencode",
	}
	for alias, want := range tests {
		got := h.resolveAlias(alias)
		if got != want {
			t.Errorf("resolveAlias(%q) = %q, want %q", alias, got, want)
		}
	}
	if got := h.resolveAlias("unknown"); got != "unknown" {
		t.Errorf("resolveAlias(unknown) = %q, want %q", got, "unknown")
	}
	h.customAliases = map[string]string{"cc": "custom-claude"}
	if got := h.resolveAlias("cc"); got != "custom-claude" {
		t.Errorf("resolveAlias(cc) with custom = %q, want custom-claude", got)
	}
}

func TestBuildHelpText(t *testing.T) {
	text := buildHelpText()
	if text == "" {
		t.Error("help text is empty")
	}
	if !strings.Contains(text, "/info") {
		t.Error("help text should mention /info")
	}
	if !strings.Contains(text, "/help") {
		t.Error("help text should mention /help")
	}
}
