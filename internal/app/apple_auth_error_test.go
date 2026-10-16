package app

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAppleAuthServerErrorIdentifiesStepWithoutHTML(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte("<html><body>503 Service Temporarily Unavailable</body></html>"))
	}))
	defer upstream.Close()
	client := NewAppleAuthClient()
	status, _, err := client.do(context.Background(), &appleAuthSession{}, http.MethodPost, upstream.URL+"/signin/init", nil, map[string]any{}, nil, false)
	if status != http.StatusServiceUnavailable || err == nil {
		t.Fatalf("status=%d err=%v", status, err)
	}
	if !strings.Contains(err.Error(), "POST /signin/init") || strings.Contains(err.Error(), "<html>") {
		t.Fatalf("unexpected error: %v", err)
	}
}
