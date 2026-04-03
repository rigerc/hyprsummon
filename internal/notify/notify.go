package notify

import (
	"errors"
	"fmt"
	"os/exec"
)

type Notifier struct{}

func New() Notifier {
	return Notifier{}
}

func (Notifier) Notify(summary string, body string) error {
	path, err := exec.LookPath("notify-send")
	if err != nil {
		return nil
	}

	cmd := exec.Command(path, summary, body)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("running notify-send: %w", err)
	}

	return nil
}

var ErrUnavailable = errors.New("notify-send unavailable")
