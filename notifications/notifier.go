package notifications

type Notifier interface {
	Notify(status, message string) error
}
