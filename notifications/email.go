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
	if cfg.SMTPServer == "" || cfg.Username == "" || cfg.Password == "" {
		return nil, fmt.Errorf("Error trying to configure email")
	}
	return &EmailNotifier{config: cfg}, nil
}

func (e *EmailNotifier) Notify(message string) error {
	if e.config.FailuresOnly && strings.Contains(message, "error") {
		return nil
	}
	finalMessage := e.config.Prefix + message
	auth := smtp.PlainAuth("", e.config.Username, e.config.Password, e.config.SMTPServer)
	msg := []byte("To: " + e.config.To + "\r\n" +
		"Subject: Backup Velero\r\n" +
		"\r\n" +
		finalMessage + 
		"\r\n")
	addr := fmt.Sprintf("%s:%d", e.config.SMTPServer, e.config.SMTPPort)
	return smtp.SendMail(addr, auth, e.config.From, []string{e.config.To}, msg)
}