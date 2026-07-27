//go:build integration

// Package testdbguard prevents PostgreSQL integration tests from opening
// databases outside the local machine.
package testdbguard

import (
	"fmt"
	"net"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Validate parses databaseURL and rejects every non-loopback connection host,
// including hosts pgx records as fallbacks for a multi-host URL.
func Validate(envName, databaseURL string) (string, *pgxpool.Config, error) {
	databaseURL = strings.TrimSpace(databaseURL)
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return "", nil, fmt.Errorf("parse %s PostgreSQL integration URL: %w", envName, err)
	}

	hosts := make([]string, 0, 1+len(config.ConnConfig.Fallbacks))
	hosts = append(hosts, config.ConnConfig.Host)
	for _, fallback := range config.ConnConfig.Fallbacks {
		if fallback == nil {
			hosts = append(hosts, "")
			continue
		}
		hosts = append(hosts, fallback.Host)
	}
	for _, rawHost := range hosts {
		host := strings.TrimSpace(rawHost)
		ip := net.ParseIP(host)
		if !strings.EqualFold(host, "localhost") && (ip == nil || !ip.IsLoopback()) {
			return "", nil, fmt.Errorf("%s must use only loopback PostgreSQL hosts, got %q", envName, host)
		}
	}

	return databaseURL, config, nil
}

// OpenValidated validates every pgx connection host before invoking open.
func OpenValidated[T any](
	envName string,
	databaseURL string,
	open func(string) (T, error),
) (T, *pgxpool.Config, error) {
	var zero T
	validatedURL, config, err := Validate(envName, databaseURL)
	if err != nil {
		return zero, nil, err
	}
	opened, err := open(validatedURL)
	if err != nil {
		return zero, nil, err
	}
	return opened, config, nil
}
