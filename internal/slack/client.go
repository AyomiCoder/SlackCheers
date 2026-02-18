package slack

import "context"

type Client interface {
	PostMessage(ctx context.Context, workspaceID, channelID, text string, avatarURLs []string) error
	SendDirectMessage(ctx context.Context, workspaceID, userID, text string) error
}
