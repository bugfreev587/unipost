package errortriage

import (
	"context"

	"github.com/xiaoboyu/unipost-api/internal/loops"
)

type loopsTransactionalClient interface {
	Enabled() bool
	SendTransactional(context.Context, loops.TransactionalEmail) error
}

type LoopsSender struct {
	client loopsTransactionalClient
}

func NewLoopsSender(client loopsTransactionalClient) *LoopsSender {
	return &LoopsSender{client: client}
}

func (s *LoopsSender) Enabled() bool {
	return s != nil && s.client != nil && s.client.Enabled()
}

func (s *LoopsSender) SendTransactional(ctx context.Context, email TransactionalEmail) error {
	return s.client.SendTransactional(ctx, loops.TransactionalEmail{
		TransactionalID: email.TransactionalID,
		Email:           email.Email,
		UserID:          email.UserID,
		IdempotencyKey:  email.IdempotencyKey,
		DataVariables:   email.DataVariables,
	})
}
