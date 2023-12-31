package pkc

import (
	"testing"

	"gotest.tools/v3/assert"
)

// TestGenerateKeyPair tests the function GenerateKeyPair
func TestGenerateKeyPair(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
	}{
		{name: "simple key pair test", wantErr: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotPrivateKey, gotPublicKey, err := GenerateKeyPair()
			if (err != nil) != tt.wantErr {
				t.Errorf("GenerateKeyPair() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			assert.Check(t, len(gotPrivateKey) > 0, "private key should not be empty", tt.name)
			assert.Check(t, len(gotPublicKey) > 0, "public key should not be empty", tt.name)
		})
	}
}
