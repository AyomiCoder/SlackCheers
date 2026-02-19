package service

import (
	"testing"

	"slackcheers/internal/domain"
)

func TestMergePeopleWithWorkspaceMembers_BuildsPeopleFromMembers(t *testing.T) {
	merged := mergePeopleWithWorkspaceMembers(nil, []dashboardWorkspaceMember{
		{ID: "U1", Handle: "alpha", DisplayName: "Alpha"},
		{ID: "U2", Handle: "beta", DisplayName: "Beta"},
	}, "W1")

	if len(merged) != 2 {
		t.Fatalf("expected 2 people, got %d", len(merged))
	}
	if merged[0].SlackUserID != "U1" || merged[1].SlackUserID != "U2" {
		t.Fatalf("unexpected people order/content: %#v", merged)
	}
	if merged[0].WorkspaceID != "W1" || merged[1].WorkspaceID != "W1" {
		t.Fatalf("expected workspace id to be set on synthetic people")
	}
}

func TestMergePeopleWithWorkspaceMembers_PreservesSavedFields(t *testing.T) {
	day := 14
	month := 6
	existing := []domain.Person{
		{
			WorkspaceID:   "W1",
			SlackUserID:   "U1",
			SlackHandle:   "alpha_saved",
			DisplayName:   "Alpha Saved",
			BirthdayDay:   &day,
			BirthdayMonth: &month,
			RemindersMode: "day_before",
		},
	}

	merged := mergePeopleWithWorkspaceMembers(existing, []dashboardWorkspaceMember{
		{ID: "U1", Handle: "alpha", DisplayName: "Alpha"},
	}, "W1")

	if len(merged) != 1 {
		t.Fatalf("expected 1 person, got %d", len(merged))
	}
	if merged[0].SlackHandle != "alpha_saved" {
		t.Fatalf("expected saved slack handle to remain, got %q", merged[0].SlackHandle)
	}
	if merged[0].DisplayName != "Alpha Saved" {
		t.Fatalf("expected saved display name to remain, got %q", merged[0].DisplayName)
	}
	if merged[0].BirthdayDay == nil || *merged[0].BirthdayDay != 14 {
		t.Fatalf("expected birthday day to remain")
	}
	if merged[0].BirthdayMonth == nil || *merged[0].BirthdayMonth != 6 {
		t.Fatalf("expected birthday month to remain")
	}
	if merged[0].RemindersMode != "day_before" {
		t.Fatalf("expected reminders mode to remain, got %q", merged[0].RemindersMode)
	}
}

func TestMergePeopleWithWorkspaceMembers_KeepsExistingMissingFromSlack(t *testing.T) {
	merged := mergePeopleWithWorkspaceMembers([]domain.Person{
		{WorkspaceID: "W1", SlackUserID: "U_STALE", DisplayName: "Stale User"},
	}, []dashboardWorkspaceMember{
		{ID: "U1", Handle: "alpha", DisplayName: "Alpha"},
	}, "W1")

	if len(merged) != 2 {
		t.Fatalf("expected 2 people, got %d", len(merged))
	}

	foundStale := false
	for _, p := range merged {
		if p.SlackUserID == "U_STALE" {
			foundStale = true
		}
	}
	if !foundStale {
		t.Fatalf("expected stale saved person to remain in list")
	}
}
