package provider

import "testing"

func TestCheckServerVersion(t *testing.T) {
	tests := []struct {
		version string
		wantErr bool
	}{
		{version: "v1.7.0", wantErr: false},
		{version: "1.7.0", wantErr: false},
		{version: "v1.7.1", wantErr: false},
		{version: "v1.8.0", wantErr: false},
		{version: "v2.0.0", wantErr: false},
		{version: "v1.6.9", wantErr: true},
		{version: "v1.0.0", wantErr: true},
		{version: "dev", wantErr: false},
		{version: "not-a-version", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.version, func(t *testing.T) {
			err := checkServerVersion(tt.version)
			if (err != nil) != tt.wantErr {
				t.Errorf("checkServerVersion(%q) error = %v, wantErr %v", tt.version, err, tt.wantErr)
			}
		})
	}
}
