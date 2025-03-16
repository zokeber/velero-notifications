package notifications

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"
)

type SlackAttachment struct {
	Fallback   string       `json:"fallback"`
	Color      string       `json:"color"`
	Pretext    string       `json:"pretext,omitempty"`
	AuthorName string       `json:"author_name,omitempty"`
	Title      string       `json:"title,omitempty"`
	Text       string       `json:"text,omitempty"`
	Fields     []SlackField `json:"fields,omitempty"`
	Footer     string       `json:"footer,omitempty"`
	Ts         int64        `json:"ts,omitempty"`
}

type SlackField struct {
	Title string `json:"title"`
	Value string `json:"value"`
	Short bool   `json:"short"`
}

type SlackConfig struct {
	Webhook      string
	Channel      string
	Username     string
	FailuresOnly bool
	Prefix       string
}

type SlackNotifier struct {
	config SlackConfig
}


func NewSlackNotifier(cfg SlackConfig) (*SlackNotifier, error) {
	if cfg.Webhook == "" {
		return nil, fmt.Errorf("empty webhook URL")
	}
	return &SlackNotifier{config: cfg}, nil
}

func (s *SlackNotifier) Notify(message string) error {

	if s.config.FailuresOnly && !strings.Contains(strings.ToLower(message), "failed") {
		return nil
	}

	finalMessage := s.config.Prefix + message

	re := regexp.MustCompile(`status:\s*(\w+)`)
	matches := re.FindStringSubmatch(finalMessage)
	backupStatus := "Unknown"
	color := "#FFF000"
	
	if len(matches) >= 2 {
		backupStatus = matches[1]
	}

	if strings.ToLower(backupStatus) == "failed" {
		color = "#FF0000" // red (failed)
	} else if strings.ToLower(backupStatus) == "partiallyfailed" {
		color = "#FFA500" // orange (partially failed)
		backupStatus = "Partially Failed"
	} else {
		color = "#36A64F" // green (success)
		backupStatus = "Completed"
	}

	attachment := SlackAttachment{
		Fallback:   finalMessage,
		Color:      color,
		Pretext:    "",
		AuthorName: s.config.Username,
		Title:      "",
		Text:       finalMessage,
		Fields: []SlackField{
			{
				Title: "Status",
				Value: backupStatus,
				Short: true,
			},
		},
		Footer: "Velero Notifications",
		Ts:     time.Now().Unix(),
	}

	payload := map[string]interface{}{
		"channel":     s.config.Channel,
		"username":    s.config.Username,
		"attachments": []SlackAttachment{attachment},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	resp, err := http.Post(s.config.Webhook, "application/json", bytes.NewBuffer(body))
	if err != nil {
		return err
	}
	
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("non-OK response from Slack: %d", resp.StatusCode)
	}
	return nil
}