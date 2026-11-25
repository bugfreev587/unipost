package worker

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/xiaoboyu/unipost-api/internal/db"
	"github.com/xiaoboyu/unipost-api/internal/handler"
	"testing"
	"time"
)

type boundedDeliveryDB struct {
	leaseNoopDB
	t     *testing.T
	calls int
}

func (d *boundedDeliveryDB) Query(ctx context.Context, _ string, _ ...interface{}) (pgx.Rows, error) {
	d.calls++
	deadline, ok := ctx.Deadline()
	if !ok || time.Until(deadline) > 10*time.Second {
		d.t.Error("delivery database operation lacks a bounded deadline")
	}
	return nil, errors.New("database unavailable")
}
func TestDeliveryDatabaseOperationsHaveDeadlines(t *testing.T) {
	for _, retry := range []bool{false, true} {
		d := &boundedDeliveryDB{t: t}
		q := db.New(d)
		h := handler.NewSocialPostHandler(q, nil, nil, nil, nil, nil, nil)
		if retry {
			NewPostRetryWorker(q, h).runOnce(context.Background())
		} else {
			NewPostDispatchWorker(q, h).runOnce(context.Background())
		}
		if d.calls < 2 {
			t.Fatalf("recovery and claim not exercised: %d", d.calls)
		}
	}
}
