package notifications

import (
	"fmt"
	"net/smtp"
	"strings"
)

type EmailNotifier struct {
	config EmailConfig
}

type EmailConfig struct {
	SMTPServer   string
	SMTPPort     int
	Username     string
	Password     string
	From         string
	To           string
	FailuresOnly bool
	Prefix       string
}

func NewEmailNotifier(cfg EmailConfig) (*EmailNotifier, error) {
	if cfg.SMTPServer == "" {
		return nil, fmt.Errorf("error trying to configure email")
	}
	return &EmailNotifier{config: cfg}, nil
}

func (e *EmailNotifier) Notify(message string) error {
	if e.config.FailuresOnly && strings.Contains(message, "failed") {
		return nil
	}

	var auth smtp.Auth
	if e.config.Username != "" && e.config.Password != "" {
		auth = smtp.PlainAuth("", e.config.Username, e.config.Password, e.config.SMTPServer)
	}

	msg := []byte("To: " + e.config.To + "\r\n" +
		"Subject: " + e.config.Prefix + " Backup Velero\r\n" +
		"\r\n" +
		message +
		"\r\n")
	addr := fmt.Sprintf("%s:%d", e.config.SMTPServer, e.config.SMTPPort)
	return smtp.SendMail(addr, auth, e.config.From, []string{e.config.To}, msg)
}
