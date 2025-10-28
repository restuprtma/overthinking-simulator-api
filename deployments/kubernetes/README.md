# Kubernetes Deployment

Kubernetes manifests untuk deploy Lakukan Backend API.

## Files

- `k8s-deployment.yaml` - Complete deployment manifest (all-in-one)

## Resources Included

### 1. ConfigMap (`lakukan-api-config`)
Non-sensitive configuration:
- Server settings (port, environment)
- Database pool configuration
- JWT expiration times
- Email settings (non-sensitive)
- Security settings

### 2. Secret (`lakukan-api-secrets`)
Sensitive data (encrypted at rest):
- Database credentials
- JWT secret key
- SMTP credentials
- Frontend URLs

**⚠️ IMPORTANT:** Update all secrets before deploying to production!

### 3. Deployment (`lakukan-api`)
Application deployment with:
- **2 replicas** (default, can be scaled)
- **Rolling update** strategy (zero downtime)
- **Resource limits**: 500m CPU, 512Mi memory
- **Health checks**: Liveness, readiness, startup probes
- **Security context**: Non-root user, dropped capabilities

### 4. Service (`lakukan-api-service`)
ClusterIP service for internal access:
- Exposes port 80 → 8080
- Load balances across pods
- Service discovery via DNS

### 5. HorizontalPodAutoscaler (`lakukan-api-hpa`)
Auto-scaling configuration:
- Min replicas: 2
- Max replicas: 10
- Scale up: CPU > 70% or Memory > 80%
- Scale down: Gradual with 5min stabilization

### 6. Ingress (`lakukan-api-ingress`)
External access with HTTPS:
- NGINX ingress controller
- Let's Encrypt SSL certificate (via cert-manager)
- SSL redirect enabled

## Quick Start

### 1. Update Secrets

**CRITICAL:** Edit `k8s-deployment.yaml` and update all secrets:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: lakukan-api-secrets
stringData:
  DB_HOST: "your-actual-postgres-host"          # ← UPDATE
  DB_PASSWORD: "your-secure-password"           # ← UPDATE
  JWT_SECRET: "your-32-char-secret-key"         # ← UPDATE
  SMTP_HOST: "smtp.gmail.com"                   # ← UPDATE
  SMTP_USER: "your-email@gmail.com"             # ← UPDATE
  SMTP_PASSWORD: "your-app-password"            # ← UPDATE
  FRONTEND_URL: "https://yourdomain.com"        # ← UPDATE
```

### 2. Update Ingress Domain

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: lakukan-api-ingress
spec:
  tls:
  - hosts:
    - api.yourdomain.com                        # ← UPDATE
  rules:
  - host: api.yourdomain.com                    # ← UPDATE
```

### 3. Deploy to Kubernetes

```bash
# Apply all resources
kubectl apply -f deployments/kubernetes/k8s-deployment.yaml

# Verify deployment
kubectl get all -l app=lakukan-api

# Check pods
kubectl get pods -l app=lakukan-api

# Check logs
kubectl logs -f deployment/lakukan-api
```

### 4. Verify Everything Works

```bash
# Check if pods are running
kubectl get pods -l app=lakukan-api
# Expected: 2 pods in Running state

# Check service
kubectl get svc lakukan-api-service
# Expected: ClusterIP service

# Check ingress
kubectl get ingress lakukan-api-ingress
# Expected: Address assigned

# Test health endpoint
kubectl port-forward svc/lakukan-api-service 8080:80
curl http://localhost:8080/health
```

## Common Operations

### View Logs

```bash
# All pods
kubectl logs -f deployment/lakukan-api

# Specific pod
kubectl logs -f lakukan-api-xxx-yyy

# Previous logs (after restart)
kubectl logs --previous lakukan-api-xxx-yyy

# Last 100 lines
kubectl logs --tail=100 deployment/lakukan-api
```

### Scale Deployment

```bash
# Manual scaling
kubectl scale deployment/lakukan-api --replicas=3

# Check scaling
kubectl get pods -l app=lakukan-api

# Auto-scaling status
kubectl get hpa lakukan-api-hpa
```

### Update Image

```bash
# Update to new image
kubectl set image deployment/lakukan-api \
  lakukan-api=your-registry/lakukan-api:v1.2.3

# Watch rollout
kubectl rollout status deployment/lakukan-api

# Check rollout history
kubectl rollout history deployment/lakukan-api
```

### Rollback Deployment

```bash
# Rollback to previous version
kubectl rollout undo deployment/lakukan-api

# Rollback to specific revision
kubectl rollout undo deployment/lakukan-api --to-revision=2

# Check rollback status
kubectl rollout status deployment/lakukan-api
```

### Update Configuration

```bash
# Edit ConfigMap
kubectl edit configmap lakukan-api-config

# Edit Secret
kubectl edit secret lakukan-api-secrets

# Restart pods to apply changes
kubectl rollout restart deployment/lakukan-api
```

### Debug Pod Issues

```bash
# Describe pod (shows events)
kubectl describe pod lakukan-api-xxx-yyy

# Get pod YAML
kubectl get pod lakukan-api-xxx-yyy -o yaml

# Execute shell in pod
kubectl exec -it lakukan-api-xxx-yyy -- sh

# Check environment variables
kubectl exec lakukan-api-xxx-yyy -- env

# Test database connection
kubectl exec -it lakukan-api-xxx-yyy -- \
  psql -h $DB_HOST -U $DB_USER -d $DB_NAME
```

## Resource Management

### Check Resource Usage

```bash
# Pod resource usage
kubectl top pods -l app=lakukan-api

# Node resource usage
kubectl top nodes

# Detailed resource requests/limits
kubectl describe deployment lakukan-api | grep -A 5 Resources
```

### Adjust Resources

Edit `k8s-deployment.yaml`:

```yaml
resources:
  requests:
    cpu: 200m        # Increase if pods are CPU throttled
    memory: 256Mi    # Increase if OOMKilled
  limits:
    cpu: 1000m       # Maximum CPU
    memory: 1Gi      # Maximum memory
```

Then apply:
```bash
kubectl apply -f deployments/kubernetes/k8s-deployment.yaml
```

## Health Checks

### Probe Configuration

```yaml
# Liveness: Is container alive?
livenessProbe:
  httpGet:
    path: /health
    port: 8080
  initialDelaySeconds: 30    # Wait before first check
  periodSeconds: 10          # Check every 10s
  failureThreshold: 3        # Restart after 3 failures

# Readiness: Is container ready for traffic?
readinessProbe:
  httpGet:
    path: /health
    port: 8080
  initialDelaySeconds: 10
  periodSeconds: 5
  failureThreshold: 3

# Startup: Special check during startup
startupProbe:
  httpGet:
    path: /health
    port: 8080
  failureThreshold: 30       # 30 * 5s = 150s max startup
  periodSeconds: 5
```

### Check Probe Status

```bash
# Check pod conditions
kubectl get pods -l app=lakukan-api -o wide

# Describe pod to see probe failures
kubectl describe pod lakukan-api-xxx-yyy | grep -A 10 Conditions
```

## Security

### Update Secrets Securely

```bash
# Create secret from file (better than inline)
kubectl create secret generic lakukan-api-secrets \
  --from-literal=DB_PASSWORD='your-secure-password' \
  --from-literal=JWT_SECRET='your-jwt-secret' \
  --dry-run=client -o yaml | kubectl apply -f -

# Or use sealed-secrets (recommended)
# 1. Install sealed-secrets controller
# 2. Create sealed secret
kubeseal --format yaml < secret.yaml > sealed-secret.yaml
# 3. Commit sealed-secret.yaml to git (safe!)
```

### Network Policies

Create network policy to restrict pod communication:

```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: lakukan-api-netpol
spec:
  podSelector:
    matchLabels:
      app: lakukan-api
  policyTypes:
  - Ingress
  - Egress
  ingress:
  - from:
    - podSelector:
        matchLabels:
          role: ingress-controller
    ports:
    - protocol: TCP
      port: 8080
  egress:
  - to:
    - podSelector:
        matchLabels:
          app: postgres
    ports:
    - protocol: TCP
      port: 5432
```

## Monitoring

### Prometheus Metrics

Add ServiceMonitor for Prometheus:

```yaml
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: lakukan-api
spec:
  selector:
    matchLabels:
      app: lakukan-api
  endpoints:
  - port: http
    path: /metrics
    interval: 30s
```

### Grafana Dashboard

Import dashboard for:
- Request rate
- Error rate
- Response time (P50, P95, P99)
- Database connection pool usage

## Troubleshooting

### Pods Not Starting

```bash
# Check pod status
kubectl get pods -l app=lakukan-api

# Common states:
# - Pending: Waiting for resources/scheduling
# - CrashLoopBackOff: Container keeps crashing
# - ImagePullBackOff: Cannot pull image
# - ErrImagePull: Image not found

# Check events
kubectl describe pod lakukan-api-xxx-yyy

# Check logs
kubectl logs lakukan-api-xxx-yyy
```

### Migration Failures

```bash
# Check if migrations ran
kubectl logs deployment/lakukan-api | grep migration

# Skip migrations (if needed)
kubectl set env deployment/lakukan-api SKIP_MIGRATION=true

# Run migrations manually
kubectl run migrate-job --rm -it --restart=Never \
  --image=your-registry/lakukan-api:latest \
  --env="DB_HOST=postgres-host" \
  --env="DB_USER=postgres" \
  --env="DB_PASSWORD=xxx" \
  --env="DB_NAME=lakukan" \
  --command -- /usr/local/bin/docker-entrypoint.sh echo "done"
```

### Service Not Accessible

```bash
# Check service endpoints
kubectl get endpoints lakukan-api-service
# Should list pod IPs

# Port forward to test
kubectl port-forward svc/lakukan-api-service 8080:80
curl http://localhost:8080/health

# Check ingress
kubectl describe ingress lakukan-api-ingress
```

## Production Checklist

Before deploying to production:

- [ ] Updated all secrets in `k8s-deployment.yaml`
- [ ] Set `DB_SSLMODE: "require"` in ConfigMap
- [ ] Updated ingress domain name
- [ ] Configured cert-manager for SSL certificates
- [ ] Set appropriate resource limits
- [ ] Configured HPA with proper thresholds
- [ ] Setup monitoring (Prometheus/Grafana)
- [ ] Setup log aggregation (ELK/Loki)
- [ ] Configured backup for database
- [ ] Setup alerts for critical metrics
- [ ] Tested rollback procedure
- [ ] Documented runbook for common issues

## Related Documentation

- [Main Deployment Docs](../README.md)
- [Docker Setup](../../README.Docker.md)
- [CI/CD Pipeline](../../.ci/README.md)
