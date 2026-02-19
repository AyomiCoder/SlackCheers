package service

import "testing"

func TestIsBotAuthoredDMMessage(t *testing.T) {
	tests := []struct {
		name      string
		msg       slackDMMessage
		botUserID string
		want      bool
	}{
		{
			name:      "bot id present",
			msg:       slackDMMessage{TS: "1.2", BotID: "B123"},
			botUserID: "U999",
			want:      true,
		},
		{
			name:      "matches bot user id",
			msg:       slackDMMessage{TS: "1.2", User: "U123"},
			botUserID: "U123",
			want:      true,
		},
		{
			name:      "bot subtype",
			msg:       slackDMMessage{TS: "1.2", Subtype: "bot_message"},
			botUserID: "",
			want:      true,
		},
		{
			name:      "missing ts",
			msg:       slackDMMessage{User: "U123", BotID: "B123"},
			botUserID: "U123",
			want:      false,
		},
		{
			name:      "user message",
			msg:       slackDMMessage{TS: "1.2", User: "U555"},
			botUserID: "U123",
			want:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isBotAuthoredDMMessage(tt.msg, tt.botUserID)
			if got != tt.want {
				t.Fatalf("unexpected result: got=%v want=%v", got, tt.want)
			}
		})
	}
}
