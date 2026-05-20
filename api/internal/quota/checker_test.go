package quota

import "testing"

func TestShouldHardBlockFreePlanQuota(t *testing.T) {
	status := QuotaStatus{Allowed: true, Usage: 99, Limit: 100}

	if !shouldHardBlockFreePlanQuota("free", status, 2) {
		t.Fatal("expected free plan to block when accepted posts would exceed quota")
	}
	if shouldHardBlockFreePlanQuota("free", status, 1) {
		t.Fatal("expected free plan to allow the final remaining post")
	}
	if shouldHardBlockFreePlanQuota("api", status, 2) {
		t.Fatal("expected paid plan to keep soft overage behavior")
	}
	if shouldHardBlockFreePlanQuota("free", QuotaStatus{Usage: 100, Limit: -1}, 1) {
		t.Fatal("expected unlimited quota to stay unblocked")
	}
}
