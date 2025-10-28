# Docker & Kubernetes Setup untuk Lakukan Backend

Dokumentasi ini menjelaskan cara menjalankan Lakukan Backend menggunakan Docker, Docker Compose, dan Kubernetes.

## Prerequisites

- Docker (versi 20.10 atau lebih baru)
- Docker Compose (versi 2.0 atau lebih baru)

## Quick Start

### 1. Setup Environment Variables

Salin file `.env.example` menjadi `.env` dan sesuaikan konfigurasi:

```bash
cp .env.example .env
```

**PENTING**: Pastikan untuk mengubah konfigurasi berikut di file `.env`:

```env
# Database (akan otomatis menggunakan container postgres)
DB_HOST=postgres  # Akan di-override oleh docker-compose

# JWT Secret (WAJIB diganti untuk production)
JWT_SECRET=ganti-dengan-secret-key-yang-aman-minimal-32-karakter

# Email SMTP (sesuaikan dengan provider email Anda)
SMTP_HOST=smtp.gmail.com
SMTP_PORT=587
SMTP_USER=your-email@gmail.com
SMTP_PASSWORD=your-app-password

# Frontend URL (sesuaikan dengan domain Anda)
FRONTEND_URL=https://yourdomain.com
EMAIL_VERIFICATION_URL=https://yourdomain.com/verify-email
RESET_PASSWORD_URL=https://yourdomain.com/reset-password
```

### 2. Build dan Jalankan Container

```bash
# Build dan jalankan semua services
docker-compose up -d

# Atau build ulang jika ada perubahan code
docker-compose up -d --build
```

### 3. Verifikasi Service Berjalan

```bash
# Cek status containers
docker-compose ps

# Lihat logs
docker-compose logs -f api

# Cek health
curl http://localhost:8080/health
```

### 4. Akses Aplikasi

- **API**: http://localhost:8080
- **Swagger Documentation**: http://localhost:8080/swagger/index.html
- **Health Check**: http://localhost:8080/health

## Docker Compose Services

### 1. PostgreSQL Database (`postgres`)
- Image: `postgres:14-alpine`
- Port: `5432` (mapped ke host)
- Volume: `postgres_data` untuk persistensi data
- Health check: setiap 10 detik

### 2. API Service (`api`)
- Port: `8080` (mapped ke host)
- Health check: setiap 30 detik
- Auto restart: unless-stopped
- **Migrations otomatis**: Dijalankan saat container start via entrypoint script
- Dependencies: postgres (healthy)

## How Migrations Work

### Docker Entrypoint

Container menggunakan `docker-entrypoint.sh` yang:
1. Menunggu database siap (dengan timeout 5 menit)
2. Menjalankan migrations otomatis:
   - Core migrations
   - CRM migrations
3. Start aplikasi

### Skip Migrations

Jika ingin skip migrations (untuk development):

```bash
# Dengan environment variable
docker-compose run -e SKIP_MIGRATION=true api

# Atau tambahkan di docker-compose.yml
environment:
  SKIP_MIGRATION: "true"
```

### Manual Migration

Jalankan migration manual:

```bash
# Jalankan container hanya untuk migration
docker-compose run --rm api sh -c "wait_for_db && run_migrations"

# Atau exec ke running container
docker-compose exec api /usr/local/bin/docker-entrypoint.sh echo "migrations done"
```

## Docker Commands

### Management

```bash
# Start services
docker-compose up -d

# Stop services
docker-compose down

# Stop dan hapus volumes (WARNING: menghapus data!)
docker-compose down -v

# Restart service tertentu
docker-compose restart api

# Rebuild service tertentu
docker-compose up -d --build api
```

### Logs

```bash
# Lihat semua logs
docker-compose logs

# Lihat logs service tertentu
docker-compose logs api
docker-compose logs postgres

# Follow logs (real-time)
docker-compose logs -f api

# Lihat 100 baris terakhir
docker-compose logs --tail=100 api
```

### Database Management

```bash
# Akses PostgreSQL shell
docker-compose exec postgres psql -U postgres -d lakukan

# Run migrations manual
docker-compose exec api migrate -path /app/internal/database/migrations/core \
  -database "postgresql://postgres:postgres@postgres:5432/lakukan?sslmode=disable&x-migrations-table=schema_migrations_core" up

# Backup database
docker-compose exec postgres pg_dump -U postgres lakukan > backup.sql

# Restore database
docker-compose exec -T postgres psql -U postgres lakukan < backup.sql
```

### Shell Access

```bash
# Akses shell di API container
docker-compose exec api sh

# Akses shell di PostgreSQL container
docker-compose exec postgres sh
```

## Production Deployment

### Security Checklist

1. **Environment Variables**
   - [ ] Ganti `JWT_SECRET` dengan random string minimal 32 karakter
   - [ ] Gunakan password database yang kuat
   - [ ] Konfigurasi SMTP dengan credentials yang benar
   - [ ] Set `ENV=production`
   - [ ] Update `FRONTEND_URL` ke domain production

2. **Database**
   - [ ] Enable SSL mode: `DB_SSLMODE=require`
   - [ ] Gunakan managed database (AWS RDS, Google Cloud SQL, dll) untuk production
   - [ ] Setup backup otomatis
   - [ ] Batasi akses database dengan firewall

3. **API**
   - [ ] Gunakan reverse proxy (Nginx, Traefik, Caddy)
   - [ ] Enable HTTPS dengan SSL certificate
   - [ ] Setup rate limiting
   - [ ] Configure CORS dengan domain yang spesifik

### Production Docker Compose Example

```yaml
# docker-compose.prod.yml
version: '3.8'

services:
  api:
    image: your-registry/lakukan-api:latest
    restart: always
    environment:
      ENV: production
      DB_SSLMODE: require
    deploy:
      replicas: 2
      resources:
        limits:
          cpus: '1'
          memory: 512M
    logging:
      driver: "json-file"
      options:
        max-size: "10m"
        max-file: "3"
```

Deploy dengan:
```bash
docker-compose -f docker-compose.prod.yml up -d
```

## Troubleshooting

### API tidak bisa connect ke database

```bash
# Cek status PostgreSQL
docker-compose ps postgres

# Cek logs PostgreSQL
docker-compose logs postgres

# Cek network connectivity
docker-compose exec api ping postgres
```

### Migration gagal

```bash
# Cek migration logs
docker-compose logs migrate

# Force migration ke version tertentu
docker-compose exec api migrate -path /app/internal/database/migrations/core \
  -database "postgresql://postgres:postgres@postgres:5432/lakukan?sslmode=disable&x-migrations-table=schema_migrations_core" \
  force VERSION_NUMBER
```

### Port sudah terpakai

Ubah port di `.env`:
```env
SERVER_PORT=8081
DB_PORT=5433
```

Lalu restart:
```bash
docker-compose down
docker-compose up -d
```

### Container terus restart

```bash
# Lihat logs untuk error
docker-compose logs api

# Cek health check
docker-compose ps
```

## Monitoring

### Health Checks

Docker Compose sudah include health checks untuk semua services:

- **PostgreSQL**: Check setiap 10s
- **API**: Check setiap 30s pada endpoint `/health`

Status bisa dilihat dengan:
```bash
docker-compose ps
```

### Resource Usage

```bash
# Lihat resource usage
docker stats

# Lihat disk usage
docker system df
```

## Cleanup

```bash
# Stop dan hapus containers (data tetap ada)
docker-compose down

# Stop, hapus containers DAN volumes (data hilang!)
docker-compose down -v

# Hapus images yang tidak terpakai
docker image prune -a

# Hapus semua (containers, networks, volumes, images)
docker system prune -a --volumes
```

## Development vs Production

### Development
- Gunakan `docker-compose.yml` langsung
- Hot reload dengan mounting volume source code
- Debug mode enabled
- Swagger accessible

### Production
- Gunakan `docker-compose.prod.yml`
- Multi-stage build untuk image yang lebih kecil
- Production environment variables
- Resource limits
- Log rotation
- Health checks
- Auto restart policies

## Kubernetes Deployment

### Prerequisites

- Kubernetes cluster (GKE, EKS, AKS, atau on-premise)
- kubectl configured
- Docker image di registry yang accessible

### Quick Deploy

```bash
# 1. Update secrets di k8s-deployment.yaml
kubectl apply -f deployments/kubernetes/k8s-deployment.yaml

# 2. Verify deployment
kubectl get pods -l app=lakukan-api
kubectl get svc lakukan-api-service

# 3. Check logs
kubectl logs -f deployment/lakukan-api

# 4. Port forward untuk testing (optional)
kubectl port-forward svc/lakukan-api-service 8080:80
```

### Kubernetes Resources

File `k8s-deployment.yaml` berisi:

1. **ConfigMap**: Non-sensitive configuration
2. **Secret**: Sensitive data (database, JWT, SMTP)
3. **Deployment**:
   - 2 replicas (default)
   - Rolling update strategy
   - Resource limits: 500m CPU, 512Mi memory
   - Health checks (liveness, readiness, startup)
4. **Service**: ClusterIP untuk internal access
5. **HorizontalPodAutoscaler**: Auto-scaling 2-10 pods based on CPU/Memory
6. **Ingress**: External access dengan HTTPS

### Update Secrets

**PENTING**: Update file `k8s-deployment.yaml` sebelum deploy:

```yaml
# Edit section Secret
stringData:
  DB_HOST: "your-actual-postgres-host"
  DB_PASSWORD: "your-secure-password"
  JWT_SECRET: "your-super-secret-key-min-32-chars"
  SMTP_HOST: "smtp.gmail.com"
  SMTP_USER: "your-email@gmail.com"
  SMTP_PASSWORD: "your-app-password"
  FRONTEND_URL: "https://yourdomain.com"
```

### Kubernetes Commands

```bash
# Get all resources
kubectl get all -l app=lakukan-api

# Scale deployment
kubectl scale deployment/lakukan-api --replicas=3

# Update image (rolling update)
kubectl set image deployment/lakukan-api lakukan-api=your-registry/lakukan-api:v1.2.3

# Rollback deployment
kubectl rollout undo deployment/lakukan-api

# Check rollout status
kubectl rollout status deployment/lakukan-api

# View logs
kubectl logs -f deployment/lakukan-api

# Execute command in pod
kubectl exec -it deployment/lakukan-api -- sh

# Describe pod for troubleshooting
kubectl describe pod -l app=lakukan-api

# Delete all resources
kubectl delete -f k8s-deployment.yaml
```

### CI/CD with Jenkins

File `Jenkinsfile` included untuk automated build & deploy:

#### Jenkins Setup

1. **Install plugins**:
   - Docker Pipeline
   - Kubernetes CLI
   - Git

2. **Configure credentials**:
   - `docker-registry-credentials`: Docker registry username/password
   - `kubernetes-credentials`: Kubeconfig file

3. **Update Jenkinsfile variables**:
```groovy
environment {
    DOCKER_REGISTRY = 'your-docker-registry.com'
    DOCKER_IMAGE = 'lakukan-api'
    K8S_NAMESPACE = 'production'
}
```

#### Jenkins Pipeline Flow

1. **Checkout**: Clone repository
2. **Build Info**: Display build metadata
3. **Build Docker Image**: Build image (NO database connection needed)
4. **Test Docker Image**: Basic validation
5. **Push to Registry**: Push to Docker registry
6. **Deploy to Kubernetes**: Update deployment with new image
7. **Verify Deployment**: Check pod health
8. **Cleanup**: Remove local images

#### Branch Strategy

- `main`/`master`: Deploy to production
- `staging`: Deploy to staging environment
- `develop`: Deploy to development environment
- Other branches: Build only (no deploy)

#### Running Jenkins Pipeline

```bash
# Create Jenkins job
# 1. New Item → Pipeline
# 2. Pipeline → Definition: Pipeline script from SCM
# 3. SCM: Git
# 4. Repository URL: your-git-repo
# 5. Script Path: Jenkinsfile
# 6. Save & Build

# Or trigger via webhook
curl -X POST https://jenkins.yourdomain.com/job/lakukan-api/build \
  --user username:token
```

### Migration in Kubernetes

Migrations otomatis berjalan saat pod start via entrypoint script:

1. Pod mulai → Entrypoint script dijalankan
2. Wait for database ready (max 5 menit)
3. Run core migrations
4. Run CRM migrations
5. Start aplikasi

**Skip migrations** jika perlu:
```yaml
# Tambahkan di Deployment env
env:
- name: SKIP_MIGRATION
  value: "true"
```

**Manual migration** di Kubernetes:
```bash
# Run migration job
kubectl run migrate-job --rm -it --restart=Never \
  --image=your-registry/lakukan-api:latest \
  --env="SKIP_MIGRATION=false" \
  --command -- /usr/local/bin/docker-entrypoint.sh echo "done"
```

## Architecture Overview

```
┌─────────────┐
│   Jenkins   │
│  (CI/CD)    │
└──────┬──────┘
       │ 1. Build & Push
       ↓
┌─────────────┐
│   Docker    │
│  Registry   │
└──────┬──────┘
       │ 2. Pull Image
       ↓
┌─────────────────────────────┐
│      Kubernetes Cluster      │
│  ┌────────────────────────┐ │
│  │   Ingress Controller    │ │
│  └───────────┬─────────────┘ │
│              ↓                │
│  ┌────────────────────────┐ │
│  │   Service (ClusterIP)   │ │
│  └───────────┬─────────────┘ │
│              ↓                │
│  ┌────────────────────────┐ │
│  │  Deployment (2+ pods)   │ │
│  │  ┌──────────────────┐  │ │
│  │  │ Pod 1 (with HPA) │  │ │
│  │  └──────────────────┘  │ │
│  │  ┌──────────────────┐  │ │
│  │  │ Pod 2 (with HPA) │  │ │
│  │  └──────────────────┘  │ │
│  └────────────────────────┘ │
│              ↓                │
│  ┌────────────────────────┐ │
│  │    PostgreSQL DB        │ │
│  │   (External/Managed)    │ │
│  └────────────────────────┘ │
└─────────────────────────────┘
```

## Best Practices

### Security
- ✅ Use secrets for sensitive data
- ✅ Run as non-root user
- ✅ Read-only root filesystem (where possible)
- ✅ Drop all capabilities
- ✅ Enable SSL/TLS for database connections in production
- ✅ Use network policies to restrict pod communication

### Performance
- ✅ Set appropriate resource requests and limits
- ✅ Use HPA for auto-scaling
- ✅ Enable connection pooling
- ✅ Use readiness/liveness probes correctly

### Reliability
- ✅ Use rolling updates (zero downtime)
- ✅ Set proper health check intervals
- ✅ Configure pod disruption budgets
- ✅ Use multiple replicas
- ✅ Implement retry logic in application

### Observability
- ✅ Centralized logging (ELK, Loki, etc.)
- ✅ Metrics collection (Prometheus)
- ✅ Distributed tracing (Jaeger, Zipkin)
- ✅ Application Performance Monitoring (APM)

## Next Steps

- ✅ Setup CI/CD pipeline dengan Jenkins (included)
- ✅ Deploy to Kubernetes (included)
- ⬜ Implement monitoring dengan Prometheus + Grafana
- ⬜ Setup centralized logging dengan ELK stack
- ⬜ Configure backup automation
- ⬜ Setup service mesh (Istio/Linkerd) untuk advanced traffic management
- ⬜ Implement GitOps dengan ArgoCD/Flux
