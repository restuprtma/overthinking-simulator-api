# CI/CD Configuration

Folder ini berisi konfigurasi untuk Continuous Integration dan Continuous Deployment.

## Files

### Jenkinsfile

Jenkins pipeline untuk automated build dan deployment.

**Pipeline Stages:**

1. **Checkout** - Clone repository
2. **Build Info** - Display build metadata (branch, commit, tag)
3. **Build Docker Image** - Build container image (NO database connection needed!)
4. **Test Docker Image** - Basic image validation
5. **Push to Registry** - Push image to Docker registry
6. **Deploy to Kubernetes** - Rolling update deployment
7. **Verify Deployment** - Check pod health
8. **Cleanup** - Remove local images

**Branch Strategy:**
- `main`/`master` → Deploy to **production**
- `staging` → Deploy to **staging**
- `develop` → Deploy to **development**
- Other branches → Build only (no deploy)

## Setup Jenkins

### 1. Install Required Plugins

- Docker Pipeline
- Kubernetes CLI
- Git Plugin

### 2. Configure Credentials

Add these credentials in Jenkins:

| ID | Type | Description |
|----|------|-------------|
| `docker-registry-credentials` | Username/Password | Docker registry credentials |
| `kubernetes-credentials` | Secret file | Kubeconfig file for K8s cluster |

### 3. Create Pipeline Job

1. New Item → Pipeline
2. Pipeline → Definition: **Pipeline script from SCM**
3. SCM: **Git**
4. Repository URL: `https://github.com/your-org/lakukan-be.git`
5. Script Path: `.ci/Jenkinsfile`
6. Save

### 4. Update Jenkinsfile Variables

Edit `.ci/Jenkinsfile`:

```groovy
environment {
    DOCKER_REGISTRY = 'your-docker-registry.com'    // Update this
    DOCKER_IMAGE = 'lakukan-api'                     // Update if needed
    K8S_NAMESPACE = 'default'                        // Update namespace
    DOCKER_CREDENTIALS_ID = 'docker-registry-credentials'
    K8S_CREDENTIALS_ID = 'kubernetes-credentials'
}
```

## Jenkins Usage

### Trigger Build Manually

1. Go to Jenkins job
2. Click "Build Now"
3. Monitor build progress
4. Check logs for any errors

### Trigger via Git Webhook

Setup webhook in your Git repository:

**GitHub:**
```
Payload URL: https://jenkins.yourdomain.com/github-webhook/
Content type: application/json
Events: Push events
```

**GitLab:**
```
URL: https://jenkins.yourdomain.com/project/lakukan-api
Secret Token: <your-token>
Trigger: Push events
```

### Trigger via API

```bash
curl -X POST "https://jenkins.yourdomain.com/job/lakukan-api/build" \
  --user "username:api-token"
```

## Pipeline Flow Diagram

```
┌─────────────┐
│  Checkout   │ ← Clone repository
└──────┬──────┘
       ↓
┌─────────────┐
│ Build Info  │ ← Show branch, commit, tag
└──────┬──────┘
       ↓
┌─────────────────────┐
│ Build Docker Image  │ ← Build image (NO DB needed!)
└──────┬──────────────┘
       ↓
┌─────────────────────┐
│ Test Docker Image   │ ← Validate image
└──────┬──────────────┘
       ↓
┌─────────────────────┐
│ Push to Registry    │ ← Push to Docker registry
└──────┬──────────────┘
       ↓
┌─────────────────────┐
│ Deploy to K8s       │ ← Rolling update (only main/staging/develop)
└──────┬──────────────┘
       ↓
┌─────────────────────┐
│ Verify Deployment   │ ← Check pod health
└──────┬──────────────┘
       ↓
┌─────────────────────┐
│    Cleanup          │ ← Remove local images
└─────────────────────┘
```

## Environment-Specific Deployment

Pipeline automatically determines deployment environment based on branch:

| Branch | Environment | K8s Namespace | Notes |
|--------|-------------|---------------|-------|
| `main` or `master` | Production | `default` | Also pushes `latest` tag |
| `staging` | Staging | `default` | Can be changed in pipeline |
| `develop` | Development | `default` | Can be changed in pipeline |
| Other branches | None | N/A | Build only, no deployment |

## Customization

### Deploy to Different Namespaces

Edit `Jenkinsfile`:

```groovy
stage('Deploy to Kubernetes') {
    steps {
        script {
            def namespace = 'default'
            if (env.BRANCH_NAME == 'staging') {
                namespace = 'staging'
            } else if (env.BRANCH_NAME == 'develop') {
                namespace = 'development'
            }

            withKubeConfig([credentialsId: K8S_CREDENTIALS_ID]) {
                sh """
                    kubectl set image deployment/lakukan-api \
                        lakukan-api=${DOCKER_REGISTRY}/${DOCKER_IMAGE}:${IMAGE_TAG} \
                        -n ${namespace}
                """
            }
        }
    }
}
```

### Add Slack Notifications

Uncomment notification sections in `Jenkinsfile`:

```groovy
post {
    success {
        slackSend(
            color: 'good',
            message: "Build Successful: ${env.JOB_NAME} #${env.BUILD_NUMBER}\nImage: ${DOCKER_REGISTRY}/${DOCKER_IMAGE}:${IMAGE_TAG}"
        )
    }
    failure {
        slackSend(
            color: 'danger',
            message: "Build Failed: ${env.JOB_NAME} #${env.BUILD_NUMBER}"
        )
    }
}
```

### Add Unit Tests Stage

Add before "Build Docker Image" stage:

```groovy
stage('Unit Tests') {
    steps {
        script {
            echo "Running unit tests..."
            sh """
                go test ./... -v -cover
            """
        }
    }
}
```

## Troubleshooting

### Build Fails at Docker Build

**Error:** `Cannot connect to Docker daemon`

**Solution:**
1. Ensure Jenkins has Docker access
2. Add Jenkins user to docker group: `sudo usermod -aG docker jenkins`
3. Restart Jenkins: `sudo systemctl restart jenkins`

### Cannot Push to Registry

**Error:** `denied: requested access to the resource is denied`

**Solution:**
1. Verify Docker registry credentials in Jenkins
2. Test login manually: `docker login your-registry.com`
3. Ensure credentials ID matches in Jenkinsfile

### Kubernetes Deployment Fails

**Error:** `error: unable to recognize "kubernetes-credentials"`

**Solution:**
1. Verify kubeconfig file is valid
2. Test kubectl: `kubectl get pods`
3. Ensure credentials ID is correct in Jenkins

### Image Tag Conflicts

**Error:** `manifest unknown`

**Solution:**
1. Verify image was pushed successfully
2. Check image name format: `registry/image:tag`
3. Ensure registry URL is correct

## Monitoring

### View Build History

```bash
# Jenkins CLI
java -jar jenkins-cli.jar -s http://jenkins.example.com/ \
  list-jobs
```

### Check Build Logs

1. Go to Jenkins job
2. Click on build number (e.g., #42)
3. Click "Console Output"

### Build Metrics

Jenkins provides:
- Build duration trends
- Success/failure rates
- Build queue statistics

## Security Best Practices

- ✅ Never commit credentials to git
- ✅ Use Jenkins credential store
- ✅ Rotate credentials regularly
- ✅ Use RBAC for Kubernetes access
- ✅ Scan Docker images for vulnerabilities
- ✅ Use signed commits
- ✅ Enable audit logging

## Related Documentation

- [Docker Setup](../README.Docker.md)
- [Deployment Files](../deployments/README.md)
- [Kubernetes Manifests](../deployments/kubernetes/)

## Support

For CI/CD issues:
1. Check Jenkins logs
2. Review pipeline console output
3. Contact DevOps team
