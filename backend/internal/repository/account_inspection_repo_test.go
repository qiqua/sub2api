package repository

import (
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

func TestAccountInspectionCandidateWhereAllowsImmediateRecheck(t *testing.T) {
	settings := service.DefaultAccountInspectionSettings()
	settings.RecheckAfterHours = 0

	where, args, err := accountInspectionCandidateWhere(settings)
	if err != nil {
		t.Fatalf("accountInspectionCandidateWhere returned error: %v", err)
	}

	if strings.Contains(where, "last_checked_at") {
		t.Fatalf("where should not filter last_checked_at when recheck window is 0: %s", where)
	}
	if len(args) != 1 {
		t.Fatalf("args length = %d, want only cursor arg", len(args))
	}
}

func TestAccountInspectionCandidateWhereSkipsRecentlyCheckedAccounts(t *testing.T) {
	settings := service.DefaultAccountInspectionSettings()
	settings.RecheckAfterHours = 168

	where, args, err := accountInspectionCandidateWhere(settings)
	if err != nil {
		t.Fatalf("accountInspectionCandidateWhere returned error: %v", err)
	}

	if !strings.Contains(where, "last_checked_at") {
		t.Fatalf("where should filter last_checked_at when recheck window is positive: %s", where)
	}
	if len(args) != 2 {
		t.Fatalf("args length = %d, want cursor and recheck args", len(args))
	}
}
