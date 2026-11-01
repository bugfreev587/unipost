package main

import (
	"os"
	"strings"
	"testing"
)

func TestFacebookPublicRoutesDoNotUseAuditOnlySuperAdminGate(t *testing.T) {
	body, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(body)
	if strings.Contains(source, "RequireFacebookSuperAdmin") {
		t.Fatal("Facebook public routes still use the audit-only super-admin gate")
	}
}
