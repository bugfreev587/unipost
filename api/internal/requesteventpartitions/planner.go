package requesteventpartitions

import (
	"errors"
	"fmt"
	"regexp"
	"time"
)

const (
	MaintenanceWeeksAhead        = 8
	MinimumCoverageDays          = 14
	PartitionLockNamespace int32 = 0x52515054
	PartitionLockKey       int32 = 1
)

var partitionSuffixPattern = regexp.MustCompile(`^[0-9]{4}w[0-9]{2}$`)

type Week struct {
	Start       time.Time
	End         time.Time
	EventTable  string
	DetailTable string
}

func PlanWeeks(now time.Time, weeksAhead int) ([]Week, error) {
	if weeksAhead < 0 {
		return nil, errors.New("weeks ahead must be nonnegative")
	}
	start := mondayUTC(now)
	result := make([]Week, 0, weeksAhead+1)
	for offset := 0; offset <= weeksAhead; offset++ {
		week, err := weekForStart(start.AddDate(0, 0, 7*offset))
		if err != nil {
			return nil, err
		}
		result = append(result, week)
	}
	return result, nil
}

func mondayUTC(value time.Time) time.Time {
	utc := value.UTC()
	day := time.Date(utc.Year(), utc.Month(), utc.Day(), 0, 0, 0, 0, time.UTC)
	offset := (int(day.Weekday()) + 6) % 7
	return day.AddDate(0, 0, -offset)
}

func weekForStart(start time.Time) (Week, error) {
	start = start.UTC()
	if start.Hour() != 0 || start.Minute() != 0 || start.Second() != 0 ||
		start.Nanosecond() != 0 || start.Weekday() != time.Monday {
		return Week{}, errors.New("week start must be Monday 00:00:00 UTC")
	}
	year, number := start.ISOWeek()
	suffix := fmt.Sprintf("%04dw%02d", year, number)
	if !partitionSuffixPattern.MatchString(suffix) {
		return Week{}, errors.New("generated partition suffix is invalid")
	}
	return Week{
		Start:       start,
		End:         start.AddDate(0, 0, 7),
		EventTable:  "api_request_events_" + suffix,
		DetailTable: "api_request_error_details_" + suffix,
	}, nil
}

func ExplicitCoverageDays(now, latestEnd time.Time) int {
	duration := latestEnd.UTC().Sub(now.UTC())
	if duration <= 0 {
		return 0
	}
	return int(duration / (24 * time.Hour))
}
