package handler

import "testing"

func TestParseSocialPostLifecyclePatch(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		wantOK  bool
		wantErr bool
		assert  func(t *testing.T, patch socialPostLifecyclePatch)
	}{
		{
			name:   "archived true",
			raw:    `{"archived":true}`,
			wantOK: true,
			assert: func(t *testing.T, patch socialPostLifecyclePatch) {
				if patch.Archived == nil || !*patch.Archived {
					t.Fatalf("expected archived=true, got %#v", patch.Archived)
				}
			},
		},
		{
			name:   "cancelled status normalized",
			raw:    `{"status":"cancelled"}`,
			wantOK: true,
			assert: func(t *testing.T, patch socialPostLifecyclePatch) {
				if patch.Status == nil || *patch.Status != "canceled" {
					t.Fatalf("expected status=canceled, got %#v", patch.Status)
				}
			},
		},
		{
			name:    "reject mixed lifecycle and content fields",
			raw:     `{"archived":true,"caption":"nope"}`,
			wantOK:  true,
			wantErr: true,
		},
		{
			name:    "reject unsupported status",
			raw:     `{"status":"published"}`,
			wantOK:  true,
			wantErr: true,
		},
		{
			name:   "regular content patch is not lifecycle",
			raw:    `{"platform_posts":[{"account_id":"sa_1","caption":"hi"}]}`,
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			patch, ok, err := parseSocialPostLifecyclePatch([]byte(tt.raw))
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOK)
			}
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if err == nil && tt.assert != nil {
				tt.assert(t, patch)
			}
		})
	}
}
