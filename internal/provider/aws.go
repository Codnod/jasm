package provider

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
)

// AWSSecretsManagerProvider implements SecretProvider for AWS Secrets Manager.
type AWSSecretsManagerProvider struct {
	client *secretsmanager.Client
}

// NewAWSSecretsManagerProvider creates a new AWS Secrets Manager provider.
// It uses the default AWS configuration which respects AWS_PROFILE environment variable.
func NewAWSSecretsManagerProvider(ctx context.Context) (*AWSSecretsManagerProvider, error) {
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	return &AWSSecretsManagerProvider{
		client: secretsmanager.NewFromConfig(cfg),
	}, nil
}

// Name returns the provider identifier.
func (p *AWSSecretsManagerProvider) Name() string {
	return "aws-secretsmanager"
}

// FetchSecret retrieves a secret from AWS Secrets Manager.
// The secret value is expected to be a JSON object with string key-value pairs.
func (p *AWSSecretsManagerProvider) FetchSecret(ctx context.Context, path string) (map[string]string, error) {
	input := &secretsmanager.GetSecretValueInput{
		SecretId: aws.String(path),
	}

	result, err := p.client.GetSecretValue(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch secret from AWS Secrets Manager: %w", err)
	}

	if result.SecretString == nil {
		return nil, fmt.Errorf("secret %s does not contain a string value", path)
	}

	// Parse JSON secret
	var rawData map[string]interface{}
	if err := json.Unmarshal([]byte(*result.SecretString), &rawData); err != nil {
		return nil, fmt.Errorf("failed to parse secret JSON: %w", err)
	}

	// Convert all values to strings
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
			// For complex types (objects, arrays), convert to JSON
			jsonBytes, _ := json.Marshal(v)
			secretData[key] = string(jsonBytes)
		}
	}

	return secretData, nil
}
