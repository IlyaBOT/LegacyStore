package api

import (
	"net/http/httptest"
	"testing"
)

func TestClientContextRejectsBotAndParsesMacBrowser(t *testing.T) {
	bot := httptest.NewRequest("POST", "https://example.test/api/v1/apps/test/view", nil)
	bot.Header.Set("User-Agent", "curl/8.0")
	if _, ok := clientContext(bot); ok {
		t.Fatal("curl user agent must not count as a browser view")
	}

	req := httptest.NewRequest("POST", "https://example.test/api/v1/apps/test/view", nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_9_5) AppleWebKit/537.36 Safari/537.36")
	got, ok := clientContext(req)
	if !ok {
		t.Fatal("normal browser user agent should be accepted")
	}
	if got.OSVersion != "10.9.5" || got.Source != "web" {
		t.Fatalf("unexpected telemetry: %#v", got)
	}
}

func TestClientContextRequiresNativeVersionAndOS(t *testing.T) {
	req := httptest.NewRequest("POST", "https://example.test/api/v1/apps/test/view", nil)
	req.Header.Set("X-LegacyStore-Client-Version", "0.1.7 beta")
	if _, ok := clientContext(req); ok {
		t.Fatal("native telemetry without OS version must be ignored")
	}
	req.Header.Set("X-LegacyStore-OS-Version", "10.6.8")
	req.Header.Set("X-LegacyStore-OS-Arch", "x86_64")
	req.Header.Set("X-LegacyStore-Device-Model", "MacBook2,1 Late 2007 (A1181)")
	got, ok := clientContext(req)
	if !ok {
		t.Fatal("identified native client should be accepted")
	}
	if got.Source != "native" || got.ClientVersion != "0.1.7 beta" || got.OSVersion != "10.6.8" || got.OSArch != "x86_64" {
		t.Fatalf("unexpected native telemetry: %#v", got)
	}
}
