# AWS Secrets Manager Examples

This directory contains complete examples for using JASM with AWS Secrets Manager.

## Table of Contents

- [Prerequisites](#prerequisites)
- [AWS Setup](#aws-setup)
- [Authentication Methods](#authentication-methods)
- [Examples](#examples)
- [Troubleshooting](#troubleshooting)

## Prerequisites

- Kubernetes cluster (EKS, k3s, or any cluster)
- AWS account with Secrets Manager access
- kubectl configured for your cluster
- JASM deployed (see [deployment guide](../../deploy/))

## AWS Setup

### 1. Create IAM Policy

Create an IAM policy for Secrets Manager access:

```bash
cat > jasm-policy.json <<EOF
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "secretsmanager:GetSecretValue",
        "secretsmanager:DescribeSecret"
      ],
      "Resource": "*"
    }
  ]
}
EOF

aws iam create-policy \
  --policy-name JASMSecretsManagerAccess \
  --policy-document file://jasm-policy.json
```

For production, restrict the `Resource` field to specific secret ARNs:

```json
{
  "Resource": [
    "arn:aws:secretsmanager:us-east-1:123456789012:secret:/prod/*",
    "arn:aws:secretsmanager:us-east-1:123456789012:secret:/staging/*"
  ]
}
```

### 2. Create Secrets in AWS

Create test secrets in AWS Secrets Manager:

```bash
# Database credentials
aws secretsmanager create-secret \
  --name /prod/myapp/database \
  --secret-string '{
    "DB_HOST": "postgres.example.com",
    "DB_PORT": "5432",
    "DB_USER": "admin",
    "DB_PASSWORD": "super-secret-password",
    "DB_NAME": "myapp_prod"
  }' \
  --region us-east-1

# API keys
aws secretsmanager create-secret \
  --name /prod/myapp/api-keys \
  --secret-string '{
    "STRIPE_API_KEY": "sk_live_...",
    "SENDGRID_API_KEY": "SG...",
    "JWT_SECRET": "random-jwt-secret"
  }' \
  --region us-east-1
```

## Authentication Methods

JASM supports three AWS authentication methods. Choose based on your environment:

### Option 1: IAM Roles for Service Accounts (IRSA) - EKS Only

**Recommended for production EKS clusters.**

#### Setup IRSA

```bash
# Create IAM role with OIDC provider
eksctl create iamserviceaccount \
  --cluster=your-cluster-name \
  --namespace=jasm \
  --name=jasm \
  --attach-policy-arn=arn:aws:iam::123456789012:policy/JASMSecretsManagerAccess \
  --approve
```

This automatically:
- Creates an IAM role with trust policy for your EKS cluster
- Annotates the service account with the role ARN
- Configures the pod to use the role

#### Verify IRSA

```bash
# Check service account annotation
kubectl -n jasm get sa jasm -o yaml | grep eks.amazonaws.com/role-arn

# Check pod has mounted token
kubectl -n jasm get pods
kubectl -n jasm exec -it <pod-name> -- ls /var/run/secrets/eks.amazonaws.com/serviceaccount/
```

### Option 2: EC2 Instance IAM Role - Self-Hosted Clusters

**Recommended for k3s and self-hosted clusters on EC2.**

Attach the IAM policy to your EC2 instance role:

```bash
# Get your instance role name
INSTANCE_PROFILE=$(aws ec2 describe-instances \
  --instance-ids i-1234567890abcdef0 \
  --query 'Reservations[0].Instances[0].IamInstanceProfile.Arn' \
  --output text | cut -d'/' -f2)

ROLE_NAME=$(aws iam get-instance-profile \
  --instance-profile-name $INSTANCE_PROFILE \
  --query 'InstanceProfile.Roles[0].RoleName' \
  --output text)

# Attach policy
aws iam attach-role-policy \
  --role-name $ROLE_NAME \
  --policy-arn arn:aws:iam::123456789012:policy/JASMSecretsManagerAccess
```

No additional configuration needed in Kubernetes - JASM will automatically use the instance role.

### Option 3: Static Credentials - Development Only

**Not recommended for production.**

Create a Kubernetes secret with AWS credentials:

```bash
kubectl create secret generic aws-credentials \
  --from-literal=AWS_ACCESS_KEY_ID="AKIA..." \
  --from-literal=AWS_SECRET_ACCESS_KEY="secret..." \
  --from-literal=AWS_REGION="us-east-1" \
  --namespace=jasm
```

Update the deployment to use the secret:

```yaml
env:
- name: AWS_REGION
  valueFrom:
    secretKeyRef:
      name: aws-credentials
      key: AWS_REGION
- name: AWS_ACCESS_KEY_ID
  valueFrom:
    secretKeyRef:
      name: aws-credentials
      key: AWS_ACCESS_KEY_ID
- name: AWS_SECRET_ACCESS_KEY
  valueFrom:
    secretKeyRef:
      name: aws-credentials
      key: AWS_SECRET_ACCESS_KEY
```

## Examples

### Example 1: Simple Pod with Database Credentials

See [test-pod.yaml](./test-pod.yaml)

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: test-app
  annotations:
    jasm.codnod.io/secret-sync: |
      provider: aws-secretsmanager
      path: /prod/myapp/database
      secretName: db-credentials
spec:
  containers:
  - name: app
    image: busybox
    command: ["sh", "-c", "env | grep DB_"]
    env:
    - name: DB_HOST
      valueFrom:
        secretKeyRef:
          name: db-credentials
          key: DB_HOST
    - name: DB_PASSWORD
      valueFrom:
        secretKeyRef:
          name: db-credentials
          key: DB_PASSWORD
```

Deploy and test:

```bash
kubectl apply -f test-pod.yaml
kubectl logs test-app
# Should output DB_HOST and DB_PASSWORD
```

### Example 2: Deployment with Multiple Secrets

See [test-deployment.yaml](./test-deployment.yaml)

This example shows:
- Deployment with 2 replicas
- Multiple secret syncs (database + API keys)
- Secrets mounted as environment variables
- Secrets mounted as files

Deploy:

```bash
kubectl apply -f test-deployment.yaml

# Verify secrets were created
kubectl get secrets | grep -E "db-credentials|api-keys"

# Check pod logs
kubectl logs -l app=test-app
```

### Example 3: Key Mapping - Rename Secret Keys

See [test-pod-with-key-mapping.yaml](./test-pod-with-key-mapping.yaml) and [test-deployment-with-key-mapping.yaml](./test-deployment-with-key-mapping.yaml)

Key mapping allows you to rename AWS secret keys when creating the Kubernetes secret. This is useful when:
- AWS secret keys don't match your application's expected environment variable names
- You want to standardize key names across different AWS secrets
- You need to transform external provider keys to internal naming conventions

#### AWS Secret Example

Create a secret with keys that don't match your app:

```bash
aws secretsmanager create-secret \
  --name database-credentials \
  --secret-string '{
    "DB_HOSTNAME": "postgres.example.com",
    "DB_PORT": "5432",
    "DB_USER": "appuser",
    "DB_PASSWORD": "secret123",
    "DB_NAME": "appdb"
  }' \
  --region us-east-1
```

#### Pod with Key Mapping

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: app-with-key-mapping
  annotations:
    jasm.codnod.io/secret-sync: |
      provider: aws-secretsmanager
      path: database-credentials
      secretName: app-db-credentials
      keys:
        host: DB_HOSTNAME
        port: DB_PORT
        database: DB_NAME
        username: DB_USER
        password: DB_PASSWORD
spec:
  containers:
  - name: app
    image: my-app:latest
    env:
    - name: DB_HOST
      valueFrom:
        secretKeyRef:
          name: app-db-credentials
          key: host
    - name: DB_PORT
      valueFrom:
        secretKeyRef:
          name: app-db-credentials
          key: port
    - name: DB_NAME
      valueFrom:
        secretKeyRef:
          name: app-db-credentials
          key: database
    - name: DB_USER
      valueFrom:
        secretKeyRef:
          name: app-db-credentials
          key: username
    - name: DB_PASSWORD
      valueFrom:
        secretKeyRef:
          name: app-db-credentials
          key: password
```

Result: The Kubernetes secret `app-db-credentials` will contain:
- `host` → value of `DB_HOSTNAME` from AWS
- `port` → value of `DB_PORT` from AWS
- `database` → value of `DB_NAME` from AWS
- `username` → value of `DB_USER` from AWS
- `password` → value of `DB_PASSWORD` from AWS

Deploy and verify:

```bash
kubectl apply -f test-pod-with-key-mapping.yaml

# Verify the secret was created with mapped keys
kubectl get secret app-db-credentials -o yaml

# Check pod can access environment variables
kubectl logs app-with-key-mapping
```

**Note**: When using key mapping, only the specified keys are included in the Kubernetes secret. Keys not in the mapping are ignored.

### Example 4: Secret as Volume Mount

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: app-with-volume
  annotations:
    jasm.codnod.io/secret-sync: |
      provider: aws-secretsmanager
      path: /prod/myapp/database
      secretName: db-config
spec:
  containers:
  - name: app
    image: nginx
    volumeMounts:
    - name: secrets
      mountPath: /etc/secrets
      readOnly: true
  volumes:
  - name: secrets
    secret:
      secretName: db-config
```

The secret files will be available at:
- `/etc/secrets/DB_HOST`
- `/etc/secrets/DB_PASSWORD`
- etc.

### Example 5: Multi-Namespace Deployment

JASM creates secrets in the same namespace as the pod:

```yaml
# Namespace: production
apiVersion: v1
kind: Pod
metadata:
  name: prod-app
  namespace: production
  annotations:
    jasm.codnod.io/secret-sync: |
      provider: aws-secretsmanager
      path: /prod/myapp/database
      secretName: db-credentials
---
# Namespace: staging
apiVersion: v1
kind: Pod
metadata:
  name: staging-app
  namespace: staging
  annotations:
    jasm.codnod.io/secret-sync: |
      provider: aws-secretsmanager
      path: /staging/myapp/database
      secretName: db-credentials
```

Each namespace gets its own `db-credentials` secret with appropriate values.

## AWS Secret Format

JASM expects secrets in JSON format with string key-value pairs:

```json
{
  "KEY_NAME": "value",
  "ANOTHER_KEY": "another-value"
}
```

Each key becomes a field in the Kubernetes secret.

### Creating Secrets

```bash
# From JSON file
aws secretsmanager create-secret \
  --name /prod/app/config \
  --secret-string file://secret.json

# Inline JSON
aws secretsmanager create-secret \
  --name /prod/app/config \
  --secret-string '{"KEY":"value"}'

# Update existing secret
aws secretsmanager update-secret \
  --secret-id /prod/app/config \
  --secret-string '{"KEY":"new-value"}'
```

## Behavior

### On Pod Start
- JASM fetches the secret from AWS Secrets Manager
- Creates/updates the Kubernetes secret in the same namespace
- Pod can now access the secret

### On Pod Restart
- Secret is refreshed from AWS Secrets Manager
- Latest values are synced to Kubernetes

### On Secret Deletion
- If you delete the Kubernetes secret, restart the pod/deployment to recreate it
- Or delete and recreate the pod

## Troubleshooting

### Secret Not Created

Check JASM logs:

```bash
kubectl -n jasm logs -l app=jasm --tail=50
```

Common issues:
- **Invalid annotation format**: Check YAML syntax in annotation
- **AWS permission denied**: Verify IAM policy and role
- **Secret not found**: Verify secret path in AWS Secrets Manager
- **Region mismatch**: Ensure AWS_REGION matches where secret is stored

### Permission Denied

Verify IAM permissions:

```bash
# For IRSA
kubectl -n jasm get sa jasm -o yaml

# Test from pod
kubectl -n jasm exec -it <jasm-pod> -- env | grep AWS

# For instance role
curl http://169.254.169.254/latest/meta-data/iam/security-credentials/
```

Check IAM policy allows:
- `secretsmanager:GetSecretValue`
- `secretsmanager:DescribeSecret`

### Secret Not Updating

JASM only syncs on pod start. To refresh:

```bash
# Restart deployment
kubectl rollout restart deployment/my-app

# Or delete pods
kubectl delete pod -l app=my-app
```

### Wrong Region

Ensure the AWS region is configured:

```bash
# Check JASM environment
kubectl -n jasm get deployment jasm -o yaml | grep AWS_REGION

# Secret must exist in same region
aws secretsmanager describe-secret \
  --secret-id /prod/myapp/database \
  --region us-east-1
```

## Best Practices

1. **Use IRSA/Instance Roles**: Avoid static credentials in production
2. **Restrict IAM permissions**: Limit access to specific secret paths
3. **Use namespaces**: Isolate secrets per environment
4. **Pin secret paths**: Use environment-specific paths (`/prod/*`, `/staging/*`)
5. **Monitor JASM logs**: Set up log aggregation for troubleshooting
6. **Test in dev first**: Verify secret sync works before production deployment
7. **Use descriptive names**: Make secret names clear and consistent
8. **Document secrets**: Keep track of which apps use which secrets

## Regional Considerations

Secrets Manager is region-specific:

```bash
# Create in multiple regions for multi-region deployments
for region in us-east-1 eu-west-1 ap-southeast-1; do
  aws secretsmanager create-secret \
    --name /prod/myapp/database \
    --secret-string '{"DB_HOST":"..."}' \
    --region $region
done
```

Configure region per cluster via configMap or environment variable.

## Cost Optimization

JASM is event-driven and only calls AWS Secrets Manager when pods start:

- **No polling**: Zero API calls when pods are stable
- **No cache**: Each pod start fetches fresh secrets
- **Low cost**: Typical monthly cost: $0.40 per secret + $0.05 per 10,000 API calls

For comparison:
- External Secrets Operator: Polls every 1-5 minutes = ~8,640-43,200 API calls/month per secret
- JASM: Only on pod start = typically <100 API calls/month per secret

## Next Steps

- Review [deployment documentation](../../deploy/README.md)
- Check [main README](../../README.md) for architecture details
- See [contributing guide](../../.github/CONTRIBUTING.md) for development setup
