package notify

import "context"

type EmailSender interface {
	Send(ctx context.Context, to string, subject string, body string) error
}

type SMSSender interface {
	Send(ctx context.Context, to string, message string) error
}
