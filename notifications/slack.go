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

	var backupStatus string
	var color string

	finalMessage := s.config.Prefix + " " + message
	lowerMsg := strings.ToLower(finalMessage)

	if s.config.FailuresOnly && !strings.Contains(lowerMsg, "failed") {
		return nil
	}

	if strings.Contains(lowerMsg, "error retrieving backups from velero") || strings.Contains(lowerMsg, "connection reset by peer") {
		backupStatus = "Failed"
	} else {
		re := regexp.MustCompile(`status:\s*(\w+)`)
		matches := re.FindStringSubmatch(finalMessage)
		if len(matches) >= 2 {
			backupStatus = matches[1]
		}
	}

	switch strings.ToLower(backupStatus) {
	case "failed":
		color = "#8B0000" // Default to dark red for failed.
		backupStatus = "Failed"
	case "partiallyfailed":
		color = "#FFA500" // Orange for partially failed.
		backupStatus = "Partially Failed"
	case "completed":
		color = "#36A64F" // Green for success.
		backupStatus = "Completed"
	case "finalizingpartiallyfailed":
		color = "#FFFF00"  // Yello for Finalizing Partially Failed
		backupStatus = "Finalizing Partially Failed"
	default:
		color = "#FF0000" // Red if status is unknown.
		backupStatus = "Unknown"
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