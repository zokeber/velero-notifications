package notifications

import (
	"fmt"
	"log"
	"net/smtp"
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

func (e *EmailNotifier) Notify(status, message string) error {
	log.Printf("[Email] Sending notification for %s: %s", status, message)
	// If FailuresOnly is enabled, only proceed for failure states
	if e.config.FailuresOnly {
		switch status {
		case "Failed", "PartiallyFailed", "FinalizingPartiallyFailed", "Unknown":

		default:
			return nil
		}
	}

	var auth smtp.Auth
	if e.config.Username != "" && e.config.Password != "" {
		auth = smtp.PlainAuth("", e.config.Username, e.config.Password, e.config.SMTPServer)
	}

	msg := []byte("To: " + e.config.To + "\r\n" +
		"Subject: " + e.config.Prefix + " Backup " + status + "\r\n" +
		"\r\n" +
		message +
		"\r\n")
	addr := fmt.Sprintf("%s:%d", e.config.SMTPServer, e.config.SMTPPort)
	return smtp.SendMail(addr, auth, e.config.From, []string{e.config.To}, msg)
}
