package worker

import (
	"reflect"
	"testing"
)

func TestParseXInboxDMCanary(t *testing.T) {
	const (
		firstID  = "11111111-1111-4111-8111-111111111111"
		secondID = "22222222-2222-4222-8222-222222222222"
	)

	tests := []struct {
		name    string
		raw     string
		want    map[string]struct{}
		wantErr bool
	}{
		{
			name: "missing value",
			raw:  "",
			want: map[string]struct{}{},
		},
		{
			name: "whitespace only",
			raw:  " \t\n ",
			want: map[string]struct{}{},
		},
		{
			name: "single UUID",
			raw:  firstID,
			want: map[string]struct{}{firstID: {}},
		},
		{
			name: "trimmed UUID list",
			raw:  "  " + firstID + " ,\t" + secondID + "  ",
			want: map[string]struct{}{firstID: {}, secondID: {}},
		},
		{
			name: "duplicate UUID",
			raw:  firstID + ", " + firstID,
			want: map[string]struct{}{firstID: {}},
		},
		{
			name:    "empty member",
			raw:     firstID + ", ," + secondID,
			want:    map[string]struct{}{},
			wantErr: true,
		},
		{
			name:    "invalid member among valid UUIDs",
			raw:     firstID + ",not-a-uuid," + secondID,
			want:    map[string]struct{}{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseXInboxDMCanary(tt.raw)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ParseXInboxDMCanary(%q) error = %v, wantErr %v", tt.raw, err, tt.wantErr)
			}
			if got == nil {
				t.Fatalf("ParseXInboxDMCanary(%q) returned a nil set", tt.raw)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("ParseXInboxDMCanary(%q) = %v, want %v", tt.raw, got, tt.want)
			}
		})
	}
}
