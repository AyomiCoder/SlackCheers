package slack

import (
	"context"
	"fmt"
	"log/slog"
)

type NoopClient struct {
	logger *slog.Logger
}

func NewNoopClient(logger *slog.Logger) *NoopClient {
	return &NoopClient{logger: logger}
}

func (c *NoopClient) PostMessage(_ context.Context, channelID, text string, avatarURLs []string) error {
	c.logger.Info("noop slack post", slog.String("channel_id", channelID), slog.String("text", text), slog.Int("avatar_count", len(avatarURLs)))
	return nil
}

func (c *NoopClient) SendDirectMessage(_ context.Context, userID, text string) error {
	c.logger.Info("noop slack dm", slog.String("user_id", userID), slog.String("text", text))
	return nil
}

func NewClient(_ string, logger *slog.Logger) (Client, error) {
	logger.Warn("using noop slack client; add a real Slack API client implementation before production")
	return NewNoopClient(logger), nil
}

func ValidatePlaceholders(template string) error {
	if template == "" {
		return fmt.Errorf("template cannot be empty")
	}
	return nil
}
