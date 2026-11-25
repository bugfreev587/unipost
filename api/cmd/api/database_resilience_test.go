package main

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5/pgxpool"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestDatabaseConnectTimeoutBounded(t *testing.T) {
	for _, initial := range []time.Duration{0, time.Second, time.Minute} {
		config, err := pgxpool.ParseConfig("postgres://postgres@localhost/test")
		if err != nil {
			t.Fatal(err)
		}
		config.ConnConfig.ConnectTimeout = initial
		configureDatabaseConnectionTimeout(config)
		if config.ConnConfig.ConnectTimeout <= 0 || config.ConnConfig.ConnectTimeout > 5*time.Second {
			t.Fatalf("unbounded timeout: %s", config.ConnConfig.ConnectTimeout)
		}
		if initial == time.Second && config.ConnConfig.ConnectTimeout != initial {
			t.Fatal("shorter configured timeout overridden")
		}
	}
}

func TestWorkerHealthReportsDatabaseFailureAndRecovery(t *testing.T) {
	failure := true
	handler := newWorkerHealthHandler(func(ctx context.Context) error {
		deadline, ok := ctx.Deadline()
		if !ok || time.Until(deadline) > 2*time.Second {
			t.Fatal("health probe has no bounded deadline")
		}
		if failure {
			return errors.New("sensitive database connection details")
		}
		return nil
	})
	for _, status := range []int{503, 200} {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, httptest.NewRequest("GET", "/health", nil))
		if w.Code != status {
			t.Fatalf("health status %d, want %d: %s", w.Code, status, w.Body.String())
		}
		if strings.Contains(w.Body.String(), "sensitive") {
			t.Fatal("connection details exposed")
		}
		failure = false
	}
}
