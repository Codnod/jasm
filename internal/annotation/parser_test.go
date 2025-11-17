package annotation

import (
	"testing"

	"k8s.io/apimachinery/pkg/types"
)

func TestParseAnnotationWithoutKeyMapping(t *testing.T) {
	annotationValue := `
provider: aws-secretsmanager
path: /prod/myapp/database
secretName: db-credentials
`

	result, err := ParseAnnotation(annotationValue, "default", "test-pod", types.UID("uid-123"))
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if result.Provider != "aws-secretsmanager" {
		t.Errorf("Expected provider 'aws-secretsmanager', got %s", result.Provider)
	}

	if result.SecretPath != "/prod/myapp/database" {
		t.Errorf("Expected path '/prod/myapp/database', got %s", result.SecretPath)
	}

	if result.SecretName != "db-credentials" {
		t.Errorf("Expected secretName 'db-credentials', got %s", result.SecretName)
	}

	if len(result.KeyMapping) != 0 {
		t.Errorf("Expected empty KeyMapping, got %v", result.KeyMapping)
	}
}

func TestParseAnnotationWithKeyMapping(t *testing.T) {
	annotationValue := `
provider: aws-secretsmanager
path: /prod/myapp/database
secretName: db-credentials
keys:
  database: DB_HOST
  password: DB_PASSWORD
  username: DB_USER
`

	result, err := ParseAnnotation(annotationValue, "default", "test-pod", types.UID("uid-123"))
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if result.Provider != "aws-secretsmanager" {
		t.Errorf("Expected provider 'aws-secretsmanager', got %s", result.Provider)
	}

	if len(result.KeyMapping) != 3 {
		t.Errorf("Expected 3 key mappings, got %d", len(result.KeyMapping))
	}

	expectedMappings := map[string]string{
		"database": "DB_HOST",
		"password": "DB_PASSWORD",
		"username": "DB_USER",
	}

	for k, v := range expectedMappings {
		if result.KeyMapping[k] != v {
			t.Errorf("Expected KeyMapping[%s]=%s, got %s", k, v, result.KeyMapping[k])
		}
	}
}

func TestParseAnnotationValidation(t *testing.T) {
	tests := []struct {
		name      string
		annotation string
		wantErr   bool
		errMsg    string
	}{
		{
			name:       "Empty annotation",
			annotation: "",
			wantErr:    true,
			errMsg:     "annotation value is empty",
		},
		{
			name:       "Missing provider",
			annotation: "path: /test\nsecretName: test",
			wantErr:    true,
			errMsg:     "provider field is required",
		},
		{
			name:       "Missing path",
			annotation: "provider: aws-secretsmanager\nsecretName: test",
			wantErr:    true,
			errMsg:     "path field is required",
		},
		{
			name:       "Missing secretName",
			annotation: "provider: aws-secretsmanager\npath: /test",
			wantErr:    true,
			errMsg:     "secretName field is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseAnnotation(tt.annotation, "default", "test-pod", types.UID("uid-123"))
			if (err == nil) != !tt.wantErr {
				t.Errorf("Unexpected error status: %v", err)
			}
		})
	}
}
