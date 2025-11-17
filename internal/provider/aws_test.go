package provider

import (
	"context"
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
