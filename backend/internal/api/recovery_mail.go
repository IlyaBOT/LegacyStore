package api

import (
	"fmt"
	"net/mail"
	"net/smtp"
	"net/url"
	"strings"

	"legacystore/backend/internal/config"
)

func sendPasswordRecoveryEmail(cfg config.Config, recipient, token string) error {
	to, err := mail.ParseAddress(strings.TrimSpace(recipient))
	if err != nil {
		return fmt.Errorf("invalid recovery recipient: %w", err)
	}
	fromValue := strings.TrimSpace(cfg.SMTPFrom)
	if fromValue == "" {
		fromValue = cfg.SMTPUsername
	}
	from, err := mail.ParseAddress(fromValue)
	if err != nil {
		return fmt.Errorf("invalid SMTP_FROM: %w", err)
	}
	baseURL := strings.TrimSpace(cfg.RecoveryBaseURL)
	if baseURL == "" {
		baseURL = strings.TrimRight(cfg.PublicBaseURL, "/") + "/account/recovery"
	}
	separator := "?"
	if strings.Contains(baseURL, "?") {
		separator = "&"
	}
	link := baseURL + separator + "token=" + url.QueryEscape(token)

	message := strings.Join([]string{
		"From: " + from.String(),
		"To: " + to.String(),
		"Subject: LegacyStore password recovery",
		"MIME-Version: 1.0",
		"Content-Type: text/plain; charset=UTF-8",
		"",
		"A password reset was requested for your LegacyStore account.",
		"",
		"Reset link:",
		link,
		"",
		"The link expires in 30 minutes and can be used once.",
		"If you did not request this, ignore this message.",
		"",
	}, "\r\n")

	address := cfg.SMTPHost + ":" + cfg.SMTPPort
	var auth smtp.Auth
	if cfg.SMTPUsername != "" {
		auth = smtp.PlainAuth("", cfg.SMTPUsername, cfg.SMTPPassword, cfg.SMTPHost)
	}
	if err := smtp.SendMail(address, auth, from.Address, []string{to.Address}, []byte(message)); err != nil {
		return fmt.Errorf("send password recovery email: %w", err)
	}
	return nil
}
