package notifications

type Notifier interface {
	Notify(message string) error
}