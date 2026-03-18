package notifications

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type SlackAttachment struct {
	Fallback string       `json:"fallback"`
	Color    string       `json:"color"`
	Blocks   []SlackBlock `json:"blocks,omitempty"`
}

type SlackTextObject struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type SlackBlock struct {
	Type     string            `json:"type"`
	Text     *SlackTextObject  `json:"text,omitempty"`
	Fields   []SlackTextObject `json:"fields,omitempty"`
	Elements []SlackTextObject `json:"elements,omitempty"`
}

type slackPayload struct {
	Text        string            `json:"text"`
	Channel     string            `json:"channel,omitempty"`
	Username    string            `json:"username,omitempty"`
	Attachments []SlackAttachment `json:"attachments,omitempty"`
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
	client *http.Client
}

type backupStateInfo struct {
	displayName string
	color       string
	emoji       string
	headerIcon  string
}

type backupMessageDetails struct {
	cluster       string
	summaryHeader string
	statusValue   string
	startTime     string
	endTime       string
	progress      string
	failureReason string
}

const slackRequestTimeout = 10 * time.Second

var statusMap = map[string]backupStateInfo{
	"failed": {
		displayName: "Failed",
		color:       "#8B0000",
		emoji:       ":x:",
		headerIcon:  "🚨",
	},
	"partiallyfailed": {
		displayName: "Partially Failed",
		color:       "#FFA500",
		emoji:       ":warning:",
		headerIcon:  "⚠️",
	},
	"completed": {
		displayName: "Completed",
		color:       "#36A64F",
		emoji:       ":white_check_mark:",
		headerIcon:  "✅",
	},
	"finalizingpartiallyfailed": {
		displayName: "Finalizing Partially Failed",
		color:       "#FFFF00",
		emoji:       ":warning:",
		headerIcon:  "⚠️",
	},
	"finalizing": {
		displayName: "Finalizing",
		color:       "#025A13",
		emoji:       ":hourglass_flowing_sand:",
		headerIcon:  "⏳",
	},
	"unknown": {
		displayName: "Unknown",
		color:       "#FF0000",
		emoji:       ":grey_question:",
		headerIcon:  "❓",
	},
}

func NewSlackNotifier(cfg SlackConfig) (*SlackNotifier, error) {
	if cfg.Webhook == "" {
		return nil, fmt.Errorf("empty webhook URL")
	}

	parsedURL, err := url.Parse(cfg.Webhook)
	if err != nil {
		return nil, fmt.Errorf("invalid webhook URL: %w", err)
	}

	if parsedURL.Scheme != "https" {
		return nil, fmt.Errorf("invalid webhook URL scheme %q: https is required", parsedURL.Scheme)
	}

	if parsedURL.Host == "" {
		return nil, fmt.Errorf("invalid webhook URL: host is required")
	}

	return &SlackNotifier{
		config: cfg,
		client: &http.Client{Timeout: slackRequestTimeout},
	}, nil
}

func (s *SlackNotifier) Notify(status, message string) error {
	finalMessage := strings.TrimSpace(strings.TrimSpace(s.config.Prefix) + " " + strings.TrimSpace(message))
	backupStatus := inferBackupStatus(status, message)

	// If FailuresOnly is enabled, only proceed for failure states
	if s.config.FailuresOnly {
		switch backupStatus {
		case "failed", "partiallyfailed", "finalizingpartiallyfailed", "unknown", "finalizing":

		default:
			return nil
		}
	}

	statusInfo := lookupStateInfo(backupStatus)
	ts := time.Now().Unix()
	attachment := SlackAttachment{
		Fallback: finalMessage,
		Color:    statusInfo.color,
		Blocks:   buildBlocks(finalMessage, statusInfo, ts, s.config.Prefix),
	}

	payload := slackPayload{
		Text:        fmt.Sprintf("%s Velero Backup Report - %s", statusInfo.headerIcon, statusInfo.displayName),
		Channel:     s.config.Channel,
		Username:    s.config.Username,
		Attachments: []SlackAttachment{attachment},
	}

	body, err := json.Marshal(payload)

	if err != nil {
		return fmt.Errorf("marshal slack payload: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, s.config.Webhook, bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("build slack request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("send slack request: %w", err)
	}

	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("non-OK response from Slack: %d", resp.StatusCode)
	}

	return nil
}

func lookupStateInfo(status string) backupStateInfo {
	statusInfo, exists := statusMap[status]
	if !exists {
		return statusMap["unknown"]
	}

	return statusInfo
}

func inferBackupStatus(status, message string) string {
	normalizedStatus := normalizeStatus(status)

	if normalizedStatus == "error" {
		return "failed"
	}

	if _, exists := statusMap[normalizedStatus]; exists {
		return normalizedStatus
	}

	lowerMsg := strings.ToLower(message)

	switch {
	case strings.Contains(lowerMsg, "error retrieving backups from velero") ||
		strings.Contains(lowerMsg, "connection reset by peer") ||
		strings.Contains(lowerMsg, "finished with status: failed"):
		return "failed"
	case strings.Contains(lowerMsg, "completed successfully"):
		return "completed"
	case strings.Contains(lowerMsg, "finished with status: partiallyfailed"):
		return "partiallyfailed"
	case strings.Contains(lowerMsg, "finished with status: finalizingpartiallyfailed"):
		return "finalizingpartiallyfailed"
	case strings.Contains(lowerMsg, "finished with status: finalizing"):
		return "finalizing"
	default:
		return "unknown"
	}
}

func normalizeStatus(status string) string {
	status = strings.TrimSpace(strings.ToLower(status))
	status = strings.ReplaceAll(status, " ", "")
	status = strings.ReplaceAll(status, "_", "")
	status = strings.ReplaceAll(status, "-", "")
	return status
}

func buildBlocks(finalMessage string, statusInfo backupStateInfo, ts int64, clusterPrefix string) []SlackBlock {
	details := parseBackupMessageDetails(finalMessage, statusInfo.displayName, clusterPrefix)
	tsString := strconv.FormatInt(ts, 10)

	blocks := []SlackBlock{
		{
			Type: "section",
			Text: &SlackTextObject{
				Type: "mrkdwn",
				Text: "*Cluster:* " + escapeMrkdwn(details.cluster),
			},
		},
		{
			Type: "section",
			Text: &SlackTextObject{
				Type: "mrkdwn",
				Text: "*" + escapeMrkdwn(details.summaryHeader) + "*\n" + escapeMrkdwn(details.statusValue),
			},
		},
	}

	if details.startTime != "" || details.endTime != "" {
		blocks = append(blocks, SlackBlock{
			Type: "section",
			Fields: []SlackTextObject{
				{
					Type: "mrkdwn",
					Text: "*Start Time:*\n" + escapeMrkdwn(details.startTime),
				},
				{
					Type: "mrkdwn",
					Text: "*End Time:*\n" + escapeMrkdwn(details.endTime),
				},
			},
		})
	}

	if details.progress != "" {
		blocks = append(blocks, SlackBlock{
			Type: "section",
			Text: &SlackTextObject{
				Type: "mrkdwn",
				Text: "*Progress:*\n" + escapeMrkdwn(details.progress),
			},
		})
	}

	if details.failureReason != "" {
		blocks = append(blocks, SlackBlock{
			Type: "section",
			Text: &SlackTextObject{
				Type: "mrkdwn",
				Text: "*Failure Reason:*\n" + escapeMrkdwn(details.failureReason),
			},
		})
	}

	blocks = append(blocks,
		SlackBlock{Type: "divider"},
		SlackBlock{
			Type: "context",
			Elements: []SlackTextObject{
				{
					Type: "mrkdwn",
					Text: "Velero Notifications | <!date^" + tsString + "^{date_long_pretty} at {time}|" + tsString + ">",
				},
			},
		},
	)

	return blocks
}

func parseBackupMessageDetails(finalMessage, fallbackStatus, clusterPrefix string) backupMessageDetails {
	details := backupMessageDetails{
		cluster: strings.TrimSpace(clusterPrefix),
	}
	if details.cluster == "" {
		details.cluster = "[cluster-unknown]"
	}

	lines := strings.Split(finalMessage, "\n")
	cleanLines := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		cleanLines = append(cleanLines, trimmed)
	}

	if len(cleanLines) == 0 {
		details.summaryHeader = "Backup status"
		details.statusValue = fallbackStatus + "."
		return details
	}

	summaryLine := cleanLines[0]
	if cluster := strings.TrimSpace(clusterPrefix); cluster != "" && strings.HasPrefix(summaryLine, cluster) {
		summaryLine = strings.TrimSpace(strings.TrimPrefix(summaryLine, cluster))
	} else if prefix := extractPrefixFromSummary(summaryLine); prefix != "" {
		details.cluster = prefix
		summaryLine = strings.TrimSpace(strings.TrimPrefix(summaryLine, prefix))
	}

	if strings.Contains(summaryLine, "finished with status:") {
		header, value, _ := strings.Cut(summaryLine, "finished with status:")
		details.summaryHeader = strings.TrimSpace(header) + " finished with status:"
		details.statusValue = strings.TrimSpace(strings.TrimSuffix(value, ".")) + "."
	} else if strings.Contains(strings.ToLower(summaryLine), "completed successfully") {
		details.summaryHeader = strings.TrimSpace(strings.TrimSuffix(summaryLine, "."))
		details.statusValue = statusMap["completed"].emoji + " Completed."
	} else {
		details.summaryHeader = "Backup status"
		details.statusValue = fallbackStatus + "."
	}

	for _, line := range cleanLines {
		if strings.HasPrefix(line, "Start Time:") {
			raw := strings.TrimSpace(strings.TrimPrefix(line, "Start Time:"))
			startRaw, endRaw, found := strings.Cut(raw, ", End Time:")
			if found {
				details.startTime = normalizeTimeDisplay(strings.TrimSpace(startRaw))
				details.endTime = normalizeTimeDisplay(strings.TrimSpace(strings.TrimSuffix(endRaw, ".")))
			}
		}

		if strings.HasPrefix(line, "Progress:") {
			details.progress = strings.TrimSpace(strings.TrimPrefix(line, "Progress:"))
		}

		if strings.HasPrefix(line, "Failure Reason:") {
			details.failureReason = strings.TrimSpace(strings.TrimPrefix(line, "Failure Reason:"))
		}
	}

	if details.summaryHeader == "" {
		details.summaryHeader = "Backup status"
	}

	if details.statusValue == "" {
		details.statusValue = fallbackStatus + "."
	}

	return details
}

func extractPrefixFromSummary(summary string) string {
	if !strings.HasPrefix(summary, "[") {
		return ""
	}

	closing := strings.Index(summary, "]")
	if closing <= 0 {
		return ""
	}

	return strings.TrimSpace(summary[:closing+1])
}

func normalizeTimeDisplay(value string) string {
	layouts := []string{
		"01/02/06 at 3:04 PM MST",
		"Mon, Jan 2, 2006 at 3:04 PM MST",
		time.RFC3339,
	}

	for _, layout := range layouts {
		parsed, err := time.Parse(layout, value)
		if err == nil {
			return parsed.Format("01/02/06 at 3:04 PM MST")
		}
	}

	return value
}

func escapeMrkdwn(input string) string {
	replacer := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
	)

	return replacer.Replace(input)
}
