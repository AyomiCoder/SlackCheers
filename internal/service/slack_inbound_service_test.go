package service

import (
	"strings"
	"testing"
)

func TestParseProfileInput_SlashBirthdayOnly(t *testing.T) {
	parsed, err := parseProfileInput("/march 25")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !parsed.HasBirthday {
		t.Fatalf("expected birthday to be parsed")
	}
	if parsed.BirthdayMon != 3 || parsed.BirthdayDay != 25 {
		t.Fatalf("unexpected birthday parsed: month=%d day=%d", parsed.BirthdayMon, parsed.BirthdayDay)
	}
	if parsed.BirthdayYr != nil {
		t.Fatalf("expected birthday year to be nil")
	}
	if parsed.HasHireDate {
		t.Fatalf("did not expect hire date")
	}
}

func TestParseProfileInput_NamedBirthdayOnly(t *testing.T) {
	parsed, err := parseProfileInput("march 25")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !parsed.HasBirthday {
		t.Fatalf("expected birthday to be parsed")
	}
	if parsed.BirthdayMon != 3 || parsed.BirthdayDay != 25 {
		t.Fatalf("unexpected birthday parsed: month=%d day=%d", parsed.BirthdayMon, parsed.BirthdayDay)
	}
	if parsed.BirthdayYr != nil {
		t.Fatalf("expected birthday year to be nil")
	}
	if parsed.HasHireDate {
		t.Fatalf("did not expect hire date")
	}
}

func TestParseProfileInput_SlashHireDateOnly(t *testing.T) {
	parsed, err := parseProfileInput("/january 23, 2024")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !parsed.HasHireDate {
		t.Fatalf("expected hire date to be parsed")
	}
	if got := parsed.HireDate.Format("2006-01-02"); got != "2024-01-23" {
		t.Fatalf("unexpected hire date parsed: %s", got)
	}
	if parsed.HasBirthday {
		t.Fatalf("did not expect birthday")
	}
}

func TestParseProfileInput_NamedHireDateOnly(t *testing.T) {
	parsed, err := parseProfileInput("january 23, 2024")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !parsed.HasHireDate {
		t.Fatalf("expected hire date to be parsed")
	}
	if got := parsed.HireDate.Format("2006-01-02"); got != "2024-01-23" {
		t.Fatalf("unexpected hire date parsed: %s", got)
	}
	if parsed.HasBirthday {
		t.Fatalf("did not expect birthday")
	}
}

func TestParseProfileInput_SlashBirthdayAndHireDate(t *testing.T) {
	parsed, err := parseProfileInput("/march 25\n/january 23, 2024")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !parsed.HasBirthday || !parsed.HasHireDate {
		t.Fatalf("expected both birthday and hire date to be parsed")
	}
	if parsed.BirthdayMon != 3 || parsed.BirthdayDay != 25 {
		t.Fatalf("unexpected birthday parsed: month=%d day=%d", parsed.BirthdayMon, parsed.BirthdayDay)
	}
	if got := parsed.HireDate.Format("2006-01-02"); got != "2024-01-23" {
		t.Fatalf("unexpected hire date parsed: %s", got)
	}
}

func TestParseProfileInput_NamedBirthdayAndHireDate(t *testing.T) {
	parsed, err := parseProfileInput("march 25\njanuary 23, 2024")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !parsed.HasBirthday || !parsed.HasHireDate {
		t.Fatalf("expected both birthday and hire date to be parsed")
	}
	if parsed.BirthdayMon != 3 || parsed.BirthdayDay != 25 {
		t.Fatalf("unexpected birthday parsed: month=%d day=%d", parsed.BirthdayMon, parsed.BirthdayDay)
	}
	if got := parsed.HireDate.Format("2006-01-02"); got != "2024-01-23" {
		t.Fatalf("unexpected hire date parsed: %s", got)
	}
}

func TestParseProfileInput_SlashDuplicateBirthday(t *testing.T) {
	_, err := parseProfileInput("/march 25\n/april 10")
	if err == nil {
		t.Fatalf("expected error for duplicate birthday lines")
	}
	if !strings.Contains(err.Error(), "multiple birthday lines") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseProfileInput_LegacyBirthdayStillWorks(t *testing.T) {
	parsed, err := parseProfileInput("birthday: 14/06")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !parsed.HasBirthday {
		t.Fatalf("expected birthday to be parsed")
	}
	if parsed.BirthdayMon != 6 || parsed.BirthdayDay != 14 {
		t.Fatalf("unexpected birthday parsed: month=%d day=%d", parsed.BirthdayMon, parsed.BirthdayDay)
	}
}

func TestBuildSaveAckMessage_BirthdayOnly(t *testing.T) {
	msg := buildSaveAckMessage(parsedProfileInput{HasBirthday: true})
	want := "Saved your birthday! Thank you for sharing with SlackCheers :yellow_heart::tada: We can't wait to celebrate you on your special day :birthday::partying_face:"
	if msg != want {
		t.Fatalf("unexpected message:\nwant: %s\ngot:  %s", want, msg)
	}
}

func TestBuildSaveAckMessage_HireDateOnly(t *testing.T) {
	msg := buildSaveAckMessage(parsedProfileInput{HasHireDate: true})
	want := "Saved your hire date! Thank you for sharing with SlackCheers :yellow_heart::tada: We can't wait to celebrate your work anniversary!"
	if msg != want {
		t.Fatalf("unexpected message:\nwant: %s\ngot:  %s", want, msg)
	}
}
