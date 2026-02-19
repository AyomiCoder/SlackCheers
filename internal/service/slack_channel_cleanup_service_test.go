package service

import (
	"strings"
	"testing"
)

func TestChannelCleanupMatchLogic(t *testing.T) {
	match := "happy birthday"
	tests := []struct {
		name      string
		msg       slackDMMessage
		botUserID string
		want      bool
	}{
		{
			name:      "bot message with birthday text",
			msg:       slackDMMessage{TS: "1.1", User: "U_BOT", Text: "🎂 Happy birthday, <@U1>!"},
			botUserID: "U_BOT",
			want:      true,
		},
		{
			name:      "bot message with non birthday text",
			msg:       slackDMMessage{TS: "1.2", User: "U_BOT", Text: "hello team"},
			botUserID: "U_BOT",
			want:      false,
		},
		{
			name:      "user message with birthday text",
			msg:       slackDMMessage{TS: "1.3", User: "U_USER", Text: "happy birthday everyone"},
			botUserID: "U_BOT",
			want:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isBotAuthoredDMMessage(tt.msg, tt.botUserID) &&
				strings.Contains(strings.ToLower(tt.msg.Text), strings.ToLower(match))
			if got != tt.want {
				t.Fatalf("unexpected match result: got=%v want=%v", got, tt.want)
			}
		})
	}
}
