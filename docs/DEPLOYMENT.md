# Deployment Guide - Lakukan Backend

Panduan lengkap untuk deployment aplikasi Lakukan Backend ke Kubernetes.

## 📁 Struktur Folder Deployment

```
lakukan-be/
├── .ci/                                    # CI/CD Configuration
│   ├── Jenkinsfile                         # Jenkins pipeline untuk automated deployment
│   └── README.md                           # CI/CD documentation
│
├── deployments/                            # Deployment Files
│   ├── README.md                           # Deployment overview
│   │
│   ├── docker/                             # Docker-related files
│   │   └── docker-entrypoint.sh            # Container entrypoint (migrations + start app)
│   │
│   └── kubernetes/                         # Kubernetes manifests
│       ├── k8s-deployment.yaml             # Complete K8s deployment (all-in-one)
│       └── README.md                       # Kubernetes deployment guide
│
├── Dockerfile                              # Docker image definition (multi-stage build)
├── .dockerignore                           # Files to exclude from Docker build
└── README.Docker.md                        # Comprehensive Docker & K8s documentation
```

## 🚀 Quick Start

### Build Docker Image

```bash
# Build image
docker build -t your-registry/lakukan-api:v1.0.0 .

# Push to registry
docker push your-registry/lakukan-api:v1.0.0
```

### Deploy to Kubernetes

```bash
# 1. Update secrets in k8s-deployment.yaml
vim deployments/kubernetes/k8s-deployment.yaml
# Update: DB_HOST, DB_PASSWORD, JWT_SECRET, SMTP credentials, etc.

# 2. Deploy to cluster
kubectl apply -f deployments/kubernetes/k8s-deployment.yaml

# 3. Verify deployment
kubectl get pods -l app=lakukan-api
kubectl logs -f deployment/lakukan-api

# 4. Check service
kubectl get svc lakukan-api-service
```

**What happens:**
- ConfigMap and Secret created
- 2 replicas deployed with rolling update
- Each pod automatically:
  - Waits for database
  - Runs migrations
  - Starts application
- Service and HPA configured
- Ingress setup for external access

### CI/CD (Jenkins)

```bash
# 1. Setup Jenkins credentials:
#    - docker-registry-credentials (Docker Hub/Registry)
#    - kubernetes-credentials (Kubeconfig)

# 2. Update Jenkinsfile variables
vim .ci/Jenkinsfile
# Update: DOCKER_REGISTRY, DOCKER_IMAGE, K8S_NAMESPACE

# 3. Create Jenkins pipeline job
#    - New Item → Pipeline
#    - Pipeline from SCM → Git
#    - Script Path: .ci/Jenkinsfile

# 4. Trigger build (push to main/staging/develop branch)
git push origin main
```

**What happens:**
1. ✅ Checkout code
2. ✅ Build Docker image (NO database needed!)
3. ✅ Test image
4. ✅ Push to registry
5. ✅ Deploy to Kubernetes (branch-based)
6. ✅ Verify deployment
7. ✅ Cleanup

## 📚 Documentation Links

### Comprehensive Guides

- **[README.Docker.md](README.Docker.md)** - Complete Docker & Kubernetes documentation
  - Kubernetes deployment
  - Troubleshooting
  - Production best practices

### Specific Topics

- **[deployments/README.md](deployments/README.md)** - Deployment files overview
- **[deployments/kubernetes/README.md](deployments/kubernetes/README.md)** - Kubernetes deployment guide
- **[.ci/README.md](.ci/README.md)** - CI/CD pipeline documentation

## 🔑 Key Features

### 1. Migration Management

**Automatic migrations** via entrypoint script:
- ✅ Runs at container startup
- ✅ Waits for database (max 5 minutes)
- ✅ Executes core + CRM migrations
- ✅ Idempotent (safe to run multiple times)

**Skip migrations** when needed:
```bash
# Kubernetes
kubectl set env deployment/lakukan-api SKIP_MIGRATION=true
```

### 2. Zero Database Connection at Build Time

**Jenkins/CI friendly:**
```bash
# Build Docker image - NO database required!
docker build -t lakukan-api:v1.0.0 .
# ✅ SUCCESS - migrations run at runtime, not build time
```

### 3. Environment-Based Deployment

**Branch → Environment mapping:**
- `main`/`master` → Production
- `staging` → Staging
- `develop` → Development
- Other branches → Build only (no deploy)

### 4. Health Checks

All environments include health checks:
- `/health` endpoint
- Liveness probe (is container alive?)
- Readiness probe (ready for traffic?)
- Startup probe (slow starting containers)

### 5. Auto-Scaling (Kubernetes)

HorizontalPodAutoscaler configured:
- Min: 2 replicas
- Max: 10 replicas
- Scale up: CPU > 70% or Memory > 80%
- Scale down: Gradual with stabilization

## 🛠 Common Operations

### View Logs

```bash
# Kubernetes
kubectl logs -f deployment/lakukan-api
```

### Scale Application

```bash
# Kubernetes manual scaling
kubectl scale deployment/lakukan-api --replicas=5

# Auto-scaling (already configured via HPA)
kubectl get hpa lakukan-api-hpa
```

### Update Image

```bash
# Kubernetes rolling update
kubectl set image deployment/lakukan-api \
  lakukan-api=your-registry/lakukan-api:v1.2.3

# Watch rollout
kubectl rollout status deployment/lakukan-api
```

### Rollback

```bash
# Kubernetes rollback
kubectl rollout undo deployment/lakukan-api

# Check history
kubectl rollout history deployment/lakukan-api
```

### Run Migrations Manually

```bash
# Kubernetes
kubectl run migrate-job --rm -it --restart=Never \
  --image=your-registry/lakukan-api:latest \
  --env="DB_HOST=postgres-host" \
  --env="DB_USER=postgres" \
  --env="DB_PASSWORD=xxx" \
  --env="DB_NAME=lakukan" \
  --command -- /usr/local/bin/docker-entrypoint.sh echo "done"
```

## 🔒 Security Checklist

Before deploying to production:

- [ ] Update `JWT_SECRET` dengan random string min 32 karakter
- [ ] Ganti semua default passwords (database, SMTP)
- [ ] Enable SSL untuk database: `DB_SSLMODE=require`
- [ ] Update `FRONTEND_URL` ke domain production
- [ ] Konfigurasi SMTP dengan credentials yang valid
- [ ] Review dan update resource limits
- [ ] Setup monitoring (Prometheus + Grafana)
- [ ] Setup log aggregation (ELK/Loki)
- [ ] Konfigurasi backup database
- [ ] Enable network policies (Kubernetes)
- [ ] Setup SSL certificate (Let's Encrypt)

## 📊 Monitoring & Observability

### Health Check

```bash
# Kubernetes
kubectl port-forward svc/lakukan-api-service 8080:80
curl http://localhost:8080/health
```

### Resource Usage

```bash
# Kubernetes
kubectl top pods -l app=lakukan-api
kubectl top nodes
```

### Application Logs

```bash
# Real-time logs
kubectl logs -f deployment/lakukan-api

# Last 100 lines
kubectl logs --tail=100 deployment/lakukan-api

# All containers
kubectl logs -f deployment/lakukan-api --all-containers=true
```

## 🐛 Troubleshooting

### Container Keeps Restarting

```bash
# Check logs
kubectl logs lakukan-api-xxx-yyy
kubectl logs --previous lakukan-api-xxx-yyy

# Check events
kubectl describe pod lakukan-api-xxx-yyy

# Common issues:
# - Database not accessible
# - Migration failed
# - Invalid environment variables
# - Health check failing
```

### Migrations Failed

```bash
# Check migration logs
kubectl logs deployment/lakukan-api | grep migration

# Run migrations manually
kubectl exec -it lakukan-api-xxx-yyy -- sh
# Inside container:
/usr/local/bin/docker-entrypoint.sh echo "done"
```

### Cannot Access API

```bash
# Check service
kubectl get svc lakukan-api-service

# Check endpoints
kubectl get endpoints lakukan-api-service

# Port forward to test
kubectl port-forward svc/lakukan-api-service 8080:80
curl http://localhost:8080/health
```

## 📞 Support

### Documentation
- [Main README](README.md) - Project overview
- [Docker Guide](README.Docker.md) - Complete Docker/K8s docs
- [Makefile](Makefile) - Build commands

### File Locations
- Dockerfile: `Dockerfile` (root)
- Kubernetes: `deployments/kubernetes/k8s-deployment.yaml`
- CI/CD: `.ci/Jenkinsfile`
- Entrypoint: `deployments/docker/docker-entrypoint.sh`

### Common Commands
```bash
# Local development
make dev                                    # Hot reload (with Air)
make db-setup                              # Setup database
make run                                   # Run application

# Build Docker image
docker build -t lakukan-api:latest .

# Kubernetes
kubectl apply -f deployments/kubernetes/k8s-deployment.yaml
kubectl get all -l app=lakukan-api
```

---

**Last Updated:** 2025-10-16
**Version:** 1.0.0
