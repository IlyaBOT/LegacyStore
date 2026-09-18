package api

import (
	"net/http"
	"regexp"
	"strings"

	"legacystore/backend/internal/account"
	"legacystore/backend/internal/catalog"
)

var macOSUserAgentPattern = regexp.MustCompile(`Mac OS X ([0-9_\.]+)`)

func clientContext(req *http.Request) (catalog.ClientTelemetry, bool) {
	ua := trimLimit(req.UserAgent(), 512)
	clientVersion := trimLimit(req.Header.Get("X-LegacyStore-Client-Version"), 64)
	osVersion := trimLimit(req.Header.Get("X-LegacyStore-OS-Version"), 32)
	osArch := strings.TrimSpace(req.Header.Get("X-LegacyStore-OS-Arch"))
	deviceModel := trimLimit(req.Header.Get("X-LegacyStore-Device-Model"), 160)

	source := "web"
	if clientVersion != "" {
		source = "native"
	}
	if osVersion == "" && source == "web" {
		osVersion = inferMacOSVersion(ua)
	}
	if osArch == "" && source == "web" {
		osArch = inferOSArch(ua)
	}
	if osArch != "i386" && osArch != "x86_64" {
		osArch = ""
	}

	telemetry := catalog.ClientTelemetry{
		Source:        source,
		UserAgent:     ua,
		ClientVersion: clientVersion,
		OSVersion:     osVersion,
		OSArch:        osArch,
		DeviceModel:   deviceModel,
	}
	if ip := clientIP(req); ip != nil {
		telemetry.IPAddress = ip.String()
	}

	if source == "native" {
		// Native clients have no browser User-Agent requirement, but must identify
		// the client build and host OS so bot-like raw HTTP requests do not count.
		return telemetry, clientVersion != "" && osVersion != ""
	}
	return telemetry, ua != "" && !isLikelyBotUserAgent(ua)
}

func (r *Router) requestTelemetry(req *http.Request) (catalog.ClientTelemetry, bool) {
	telemetry, acceptable := clientContext(req)
	if !acceptable {
		return telemetry, false
	}
	if r.users == nil || sessionToken(req) == "" || !isSecureRequest(req) {
		return telemetry, true
	}

	user, session, err := r.users.UserBySession(req.Context(), sessionToken(req), clientIP(req), req.UserAgent())
	if err != nil {
		return telemetry, true
	}
	telemetry.UserID = user.ID
	telemetry.UsernameSnapshot = user.Nickname
	if strings.TrimSpace(telemetry.UsernameSnapshot) == "" {
		telemetry.UsernameSnapshot = user.Email
	}
	if session.AuthKind == "legacy" && telemetry.ClientVersion == "" {
		telemetry.Source = "legacy"
	}
	return telemetry, true
}

func reviewContextFromRequest(req *http.Request, payload reviewPayload, session *account.Session) account.ReviewContext {
	telemetry, _ := clientContext(req)
	if payload.OSVersion != "" {
		telemetry.OSVersion = trimLimit(payload.OSVersion, 32)
	}
	if payload.OSArch != "" {
		telemetry.OSArch = strings.TrimSpace(payload.OSArch)
	}
	if payload.DeviceModel != "" {
		telemetry.DeviceModel = trimLimit(payload.DeviceModel, 160)
	}
	if payload.ClientVersion != "" {
		telemetry.ClientVersion = trimLimit(payload.ClientVersion, 64)
	}
	source := telemetry.Source
	if session != nil && session.AuthKind == "legacy" && telemetry.ClientVersion == "" {
		source = "legacy"
	}
	return account.ReviewContext{
		AppVersion:    strings.TrimSpace(payload.AppVersion),
		OSVersion:     telemetry.OSVersion,
		OSArch:        telemetry.OSArch,
		DeviceModel:   telemetry.DeviceModel,
		ClientVersion: telemetry.ClientVersion,
		Source:        source,
	}
}

func inferMacOSVersion(userAgent string) string {
	match := macOSUserAgentPattern.FindStringSubmatch(userAgent)
	if len(match) != 2 {
		return ""
	}
	return strings.ReplaceAll(match[1], "_", ".")
}

func inferOSArch(userAgent string) string {
	lower := strings.ToLower(userAgent)
	switch {
	case strings.Contains(lower, "x86_64"), strings.Contains(lower, "x86-64"):
		return "x86_64"
	case strings.Contains(lower, "i386"), strings.Contains(lower, "i686"):
		return "i386"
	default:
		return ""
	}
}

func isLikelyBotUserAgent(userAgent string) bool {
	lower := strings.ToLower(strings.TrimSpace(userAgent))
	if lower == "" {
		return true
	}
	patterns := []string{
		"bot", "crawler", "spider", "slurp", "curl/", "wget/",
		"python-requests", "go-http-client", "scrapy", "libwww-perl",
		"headlesschrome", "feedfetcher",
	}
	for _, pattern := range patterns {
		if strings.Contains(lower, pattern) {
			return true
		}
	}
	return false
}

func trimLimit(value string, limit int) string {
	value = strings.TrimSpace(value)
	if len(value) <= limit {
		return value
	}
	return value[:limit]
}
