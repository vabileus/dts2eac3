package main

// Notifier sends best-effort native desktop notifications.
// Failures are intentionally swallowed: a missing notifier must never break
// the conversion run.
type Notifier struct{ enabled bool }

func (n Notifier) Send(title, body string) {
	if !n.enabled {
		return
	}
	_ = nativeNotify(title, body) // platform-specific, best effort
}
