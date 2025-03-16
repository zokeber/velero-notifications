package main

import (
	"context"
	"flag"
	"log"
	"time"

	"github.com/zokeber/velero-notifications/config"
	"github.com/zokeber/velero-notifications/controllers"
	"github.com/zokeber/velero-notifications/notifications"

)

func main() {

	configPath := flag.String("config", "config/config.yaml", "Path of config.yaml file")
	flag.Parse()

	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		log.Fatalf("Failed to retrieve the config.yaml file: %v", err)
	}

	var notifiers []notifications.Notifier
	
	if cfg.Notifications.Slack.Enabled {
		slackNotifier, err := notifications.NewSlackNotifier(notifications.SlackConfig{
			Webhook:      cfg.Notifications.Slack.Webhook,
			Channel:      cfg.Notifications.Slack.Channel,
			Username:     cfg.Notifications.Slack.Username,
			FailuresOnly: cfg.Notifications.Slack.FailuresOnly,
			Prefix:       cfg.Notifications.NotificationPrefix,
		})
		if err != nil {
			log.Printf("Failed to initialize Slack notifier: %v", err)
		} else {
			notifiers = append(notifiers, slackNotifier)
		}
	}

	if cfg.Notifications.Email.Enabled {
		emailNotifier, err := notifications.NewEmailNotifier(notifications.EmailConfig{
			SMTPServer:   cfg.Notifications.Email.SMTPServer,
			SMTPPort:     cfg.Notifications.Email.SMTPPort,
			Username:     cfg.Notifications.Email.Username,
			Password:     cfg.Notifications.Email.Password,
			From:         cfg.Notifications.Email.From,
			To:           cfg.Notifications.Email.To,
			FailuresOnly: cfg.Notifications.Email.FailuresOnly,
			Prefix:       cfg.Notifications.NotificationPrefix,
		})
		if err != nil {
			log.Printf("Failed to initialize Email notifier: %v", err)
		} else {
			notifiers = append(notifiers, emailNotifier)
		}
	}

	veleroController, err := controller.NewVeleroController(
		cfg.Namespace,
		cfg.CheckInterval,
		cfg.Logging.Verbose,
		notifiers,
	)
	
	if err != nil {
		log.Fatalf("Unable to initialize Velero Controller: %v", err)
	}

	ctx := context.Background()
	go veleroController.Run(ctx)

	select {
	case <-ctx.Done():
		log.Println("Exit")
		time.Sleep(2 * time.Second)
	}
}