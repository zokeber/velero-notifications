package notifications

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
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

type backupStateInfo struct {
	displayName string
	color       string
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

	// Map of status types to their display properties
	statusMap := map[string]backupStateInfo{
		"failed": {
			displayName: "Failed",
			color:       "#8B0000", // Dark red for failed state
		},
		"partiallyfailed": {
			displayName: "Partially Failed",
			color:       "#FFA500", // Orange for partially failed state
		},
		"completed": {
			displayName: "Completed",
			color:       "#36A64F", // Green for completed state
		},
		"finalizingpartiallyfailed": {
			displayName: "Finalizing Partially Failed",
			color:       "#FFFF00", // Yellow for finalizing partially failed state
		},
		"unknown": {
			displayName: "Unknown",
			color:       "#FF0000", // Red for unknown state
		},
	}

	// Determine backup status based on message content
	switch {
	case strings.Contains(lowerMsg, "error retrieving backups from velero") ||
		strings.Contains(lowerMsg, "connection reset by peer") ||
		strings.Contains(lowerMsg, "finished with status: failed"):
		backupStatus = "failed"
	case strings.Contains(lowerMsg, "completed successfully"):
		backupStatus = "completed"
	case strings.Contains(lowerMsg, "finished with status: partiallyfailed"):
		backupStatus = "partiallyfailed"
	case strings.Contains(lowerMsg, "finished with status: finalizingpartiallyfailed"):
		backupStatus = "finalizingpartiallyfailed"
	default:
		backupStatus = "unknown"
	}

	// If FailuresOnly is enabled, only proceed for failure states
	if s.config.FailuresOnly {
		switch backupStatus {
		case "failed", "partiallyfailed", "finalizingpartiallyfailed", "unknown":

		default:
			return nil
		}
	}

	// Get status info from the map, defaulting to unknown if not found
	statusInfo, exists := statusMap[strings.ToLower(backupStatus)]
	if !exists {
		statusInfo = statusMap["unknown"]
	}

	backupStatus = statusInfo.displayName
	color = statusInfo.color

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
