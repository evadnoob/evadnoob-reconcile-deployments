package freeport

import (
	"testing"

	"gotest.tools/v3/assert"
)

// TestGetFreePort tests the function GetFreePort
func TestGetFreePort(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
	}{
		{name: "simple test get free port", wantErr: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetFreePort()
			if (err != nil) != tt.wantErr {
				t.Errorf("GetFreePort() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			assert.Assert(t, got > 0, "port should be greater than 0", tt.name)
		})
	}
}
