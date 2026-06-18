package changelog

import (
	"testing"
	"time"
)

func TestSignerVerifiesValidSignature(t *testing.T) {
	signer := NewSigner("secret")
	signer.now = func() time.Time { return time.Unix(1000, 0).UTC() }
	expires := time.Unix(1200, 0).UTC()

	signature := signer.Sign("candidate-1", ActionPublish, expires, "source-hash")

	if err := signer.Verify("candidate-1", ActionPublish, expires.Unix(), "source-hash", signature); err != nil {
		t.Fatalf("Verify returned %v, want nil", err)
	}
}

func TestSignerRejectsTamperedActionAndExpiredLinks(t *testing.T) {
	signer := NewSigner("secret")
	signer.now = func() time.Time { return time.Unix(1000, 0).UTC() }
	expires := time.Unix(1001, 0).UTC()
	signature := signer.Sign("candidate-1", ActionPublish, expires, "source-hash")

	if err := signer.Verify("candidate-1", ActionDiscard, expires.Unix(), "source-hash", signature); err == nil {
		t.Fatal("Verify accepted a signature for a different action")
	}

	signer.now = func() time.Time { return time.Unix(1002, 0).UTC() }
	if err := signer.Verify("candidate-1", ActionPublish, expires.Unix(), "source-hash", signature); err == nil {
		t.Fatal("Verify accepted an expired signature")
	}
}
