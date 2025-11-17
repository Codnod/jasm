package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
)

func TestAWSSecretsManagerProvider_Name(t *testing.T) {
	provider := &AWSSecretsManagerProvider{}
	if got := provider.Name(); got != "aws-secretsmanager" {
		t.Errorf("Name() = %v, want %v", got, "aws-secretsmanager")
	}
}

// Note: Full integration tests for AWS provider require actual AWS credentials
// and are better suited for E2E tests. These unit tests verify the structure.
func TestAWSSecretsManagerProvider_Structure(t *testing.T) {
	// Verify the provider implements the SecretProvider interface
	var _ SecretProvider = (*AWSSecretsManagerProvider)(nil)
}

func TestValueConversion(t *testing.T) {
	tests := []struct {
		name     string
		jsonData string
		want     map[string]string
		wantErr  bool
	}{
		{
			name:     "All string values",
			jsonData: `{"host":"localhost","password":"secret123"}`,
			want:     map[string]string{"host": "localhost", "password": "secret123"},
			wantErr:  false,
		},
		{
			name:     "Numeric port values",
			jsonData: `{"host":"localhost","port":5432,"timeout":30}`,
			want:     map[string]string{"host": "localhost", "port": "5432", "timeout": "30"},
			wantErr:  false,
		},
		{
			name:     "Boolean values",
			jsonData: `{"host":"localhost","ssl":true,"verify":false}`,
			want:     map[string]string{"host": "localhost", "ssl": "true", "verify": "false"},
			wantErr:  false,
		},
		{
			name:     "Null values",
			jsonData: `{"host":"localhost","optional":null}`,
			want:     map[string]string{"host": "localhost", "optional": ""},
			wantErr:  false,
		},
		{
			name:     "Mixed types",
			jsonData: `{"username":"admin","password":"p@ss123","port":3306,"ssl":true,"timeout":null}`,
			want:     map[string]string{"username": "admin", "password": "p@ss123", "port": "3306", "ssl": "true", "timeout": ""},
			wantErr:  false,
		},
		{
			name:     "Invalid JSON",
			jsonData: `{invalid json}`,
			want:     nil,
			wantErr:  true,
		},
		{
			name:     "JSON array values",
			jsonData: `{"hosts":["host1","host2"],"ports":[5432,3306]}`,
			want:     map[string]string{"hosts": `["host1","host2"]`, "ports": "[5432,3306]"},
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simulate the conversion logic from FetchSecret
			var rawData map[string]interface{}
			if err := json.Unmarshal([]byte(tt.jsonData), &rawData); err != nil {
				if !tt.wantErr {
					t.Fatalf("json.Unmarshal() error = %v, want nil", err)
				}
				return
			}

			// Convert all values to strings (same logic as in FetchSecret)
			secretData := make(map[string]string, len(rawData))
			for key, value := range rawData {
				switch v := value.(type) {
				case string:
					secretData[key] = v
				case float64:
					secretData[key] = fmt.Sprintf("%v", v)
				case bool:
					secretData[key] = fmt.Sprintf("%v", v)
				case nil:
					secretData[key] = ""
				default:
					jsonBytes, _ := json.Marshal(v)
					secretData[key] = string(jsonBytes)
				}
			}

			// Compare results
			if len(secretData) != len(tt.want) {
				t.Errorf("got %d keys, want %d keys", len(secretData), len(tt.want))
			}

			for k, v := range tt.want {
				if got, ok := secretData[k]; !ok {
					t.Errorf("missing key %q", k)
				} else if got != v {
					t.Errorf("key %q: got %q, want %q", k, got, v)
				}
			}
		})
	}
}

func TestNewAWSSecretsManagerProvider(t *testing.T) {
	// This test will fail if AWS credentials are not configured
	// Skip in CI environments without AWS access
	t.Skip("Skipping AWS provider creation test - requires AWS credentials")

	ctx := context.Background()
	provider, err := NewAWSSecretsManagerProvider(ctx)
	if err != nil {
		t.Fatalf("NewAWSSecretsManagerProvider() error = %v", err)
	}
	if provider == nil {
		t.Fatal("NewAWSSecretsManagerProvider() returned nil provider")
	}
	if provider.client == nil {
		t.Error("NewAWSSecretsManagerProvider() client is nil")
	}
}
