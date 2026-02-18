package slack

import "context"

type Client interface {
	PostMessage(ctx context.Context, channelID, text string, avatarURLs []string) error
	SendDirectMessage(ctx context.Context, userID, text string) error
}
