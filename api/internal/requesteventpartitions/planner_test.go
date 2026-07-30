package requesteventpartitions

import (
	"testing"
	"time"
)

func TestPlanWeeksUsesISOWeekYearAndEightWeekHorizon(t *testing.T) {
	got, err := PlanWeeks(time.Date(2026, 12, 31, 23, 0, 0, 0, time.UTC), 8)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 9 {
		t.Fatalf("weeks = %d, want 9", len(got))
	}
	if got[0].Start != time.Date(2026, 12, 28, 0, 0, 0, 0, time.UTC) {
		t.Fatalf("first start = %v", got[0].Start)
	}
	if got[0].End != time.Date(2027, 1, 4, 0, 0, 0, 0, time.UTC) {
		t.Fatalf("first end = %v", got[0].End)
	}
	if got[0].EventTable != "api_request_events_2026w53" {
		t.Fatalf("first event table = %q", got[0].EventTable)
	}
	if got[0].DetailTable != "api_request_error_details_2026w53" {
		t.Fatalf("first detail table = %q", got[0].DetailTable)
	}
	if got[1].EventTable != "api_request_events_2027w01" {
		t.Fatalf("second event table = %q", got[1].EventTable)
	}
}

func TestPlanWeeksNormalizesInputToUTC(t *testing.T) {
	location := time.FixedZone("UTC+14", 14*60*60)
	now := time.Date(2027, 1, 4, 1, 0, 0, 0, location)
	got, err := PlanWeeks(now, 0)
	if err != nil {
		t.Fatal(err)
	}
	want := time.Date(2026, 12, 28, 0, 0, 0, 0, time.UTC)
	if got[0].Start != want {
		t.Fatalf("start = %v, want %v", got[0].Start, want)
	}
}

func TestPlanWeeksRejectsNegativeHorizon(t *testing.T) {
	if _, err := PlanWeeks(time.Now(), -1); err == nil {
		t.Fatal("negative horizon must fail")
	}
}

func TestExplicitCoverageDaysUsesWholeFutureDays(t *testing.T) {
	now := time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC)
	end := time.Date(2026, 8, 17, 0, 0, 0, 0, time.UTC)
	if got := ExplicitCoverageDays(now, end); got != 18 {
		t.Fatalf("coverage = %d, want 18", got)
	}
	if got := ExplicitCoverageDays(end, now); got != 0 {
		t.Fatalf("past coverage = %d, want 0", got)
	}
}

func TestPartitionSuffixRejectsInvalidIdentifiers(t *testing.T) {
	if _, err := weekForStart(time.Date(2026, 7, 28, 0, 0, 0, 0, time.UTC)); err == nil {
		t.Fatal("non-Monday boundary must fail")
	}
	if _, err := weekForStart(time.Date(2026, 7, 27, 0, 0, 0, 1, time.UTC)); err == nil {
		t.Fatal("non-midnight boundary must fail")
	}
}
