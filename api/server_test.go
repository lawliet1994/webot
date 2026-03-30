package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandleSend_AcceptsMediaPath(t *testing.T) {
	s := &Server{}

	req := httptest.NewRequest(http.MethodPost, "/api/send", strings.NewReader(`{"to":"user_id@im.wechat","media_path":"/tmp/photo.png"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	s.handleSend(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status %d, got %d, body=%q", http.StatusServiceUnavailable, w.Code, w.Body.String())
	}
}
