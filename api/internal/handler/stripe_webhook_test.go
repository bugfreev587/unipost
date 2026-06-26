package handler

import "testing"

func TestShouldSendBillingPaymentRecovered(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		status string
		want   bool
	}{
		{name: "past due recovers", status: "past_due", want: true},
		{name: "active replay does not resend", status: "active", want: false},
		{name: "empty status does not send", status: "", want: false},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := shouldSendBillingPaymentRecovered(tc.status); got != tc.want {
				t.Fatalf("shouldSendBillingPaymentRecovered(%q) = %v, want %v", tc.status, got, tc.want)
			}
		})
	}
}
