# GlobeCo Portfolio Accounting Service - Kubernetes Deployment

This directory contains comprehensive Kubernetes manifests for deploying the GlobeCo Portfolio Accounting Service to Kubernetes 1.33+ clusters.

## Architecture Overview

The deployment includes:
- **Portfolio Accounting Service**: Main microservice with auto-scaling
- **PostgreSQL Database**: Persistent data storage with high availability
- **Hazelcast Cache Cluster**: Distributed caching for performance
- **Network Policies**: Security isolation and traffic control
- **Ingress Controller**: External access with TLS termination
- **Monitoring Integration**: Prometheus metrics and health checks

## Prerequisites

### Required Tools
- `kubectl` (v1.28+)
- `kustomize` (v5.0+) or `kubectl kustomize`
- `helm` (v3.10+) - optional for additional services
- Docker (for building images)

### Cluster Requirements
- Kubernetes 1.33+ cluster
- Ingress controller (nginx recommended)
- StorageClass for persistent volumes
- CNI with NetworkPolicy support
- Metrics server for HPA

### Optional Components
- Cert-manager for TLS certificates
- Prometheus Operator for monitoring
- Service mesh (Linkerd/Istio)

## Quick Start

### 1. Deploy with Script

```bash
# Deploy with default configuration
./scripts/k8s-deploy.sh deploy

# Deploy with custom configuration
./scripts/k8s-deploy.sh deploy -e staging -t v1.2.3 -r 5

# Check deployment status
./scripts/k8s-deploy.sh status
```

### 2. Manual Deployment

```bash
# Create namespace
kubectl apply -f namespace.yaml

# Apply configuration and secrets
kubectl apply -f configmap.yaml
kubectl apply -f secrets.yaml

# Deploy infrastructure
kubectl apply -f postgres.yaml
kubectl apply -f hazelcast.yaml

# Deploy application
kubectl apply -f deployment.yaml
kubectl apply -f service.yaml
kubectl apply -f hpa.yaml

# Configure networking
kubectl apply -f network-policy.yaml
kubectl apply -f ingress.yaml
```

### 3. Using Kustomize

```bash
# Deploy with kustomize
kubectl apply -k .

# Deploy with custom overlays
kubectl apply -k overlays/staging
```

## File Descriptions

### Core Manifests

| File | Description | Purpose |
|------|-------------|---------|
| `namespace.yaml` | Namespace, ResourceQuota, LimitRange | Resource isolation and limits |
| `configmap.yaml` | Application and Hazelcast configuration | Non-sensitive configuration |
| `secrets.yaml` | Database credentials, API keys, certificates | Sensitive configuration |
| `deployment.yaml` | Main application deployment with ServiceAccount | Core application runtime |
| `service.yaml` | Service definitions and ServiceMonitor | Network access and monitoring |
| `postgres.yaml` | PostgreSQL deployment with persistence | Database infrastructure |
| `hazelcast.yaml` | Hazelcast StatefulSet with RBAC | Distributed caching |
| `hpa.yaml` | Horizontal and Vertical Pod Autoscalers | Auto-scaling configuration |
| `ingress.yaml` | Ingress, HTTPRoute, and Certificate | External access and TLS |
| `network-policy.yaml` | NetworkPolicies for security | Network segmentation |

### Utility Files

| File | Description |
|------|-------------|
| `kustomization.yaml` | Kustomize configuration |
| `README.md` | This documentation |

## Configuration

### Environment Variables

The deployment can be customized using environment variables:

```bash
export NAMESPACE="globeco-portfolio-accounting"
export ENVIRONMENT="production"
export IMAGE_TAG="v1.0.0"
export REPLICAS="3"
```

### ConfigMap Customization

Key configuration parameters in `configmap.yaml`:

```yaml
# Database settings
DB_MAX_OPEN_CONNS: "25"
DB_MAX_IDLE_CONNS: "10"

# Cache settings
CACHE_TTL: "3600"
CACHE_CLUSTER_NAME: "portfolio-accounting-cluster"

# Monitoring
METRICS_ENABLED: "true"
TRACING_ENABLED: "true"
```

### Secrets Management

Update secrets in `secrets.yaml` for production:

```bash
# Generate base64 encoded values
echo -n "your-password" | base64

# Update secrets
kubectl create secret generic portfolio-accounting-secrets \
  --from-literal=DB_PASSWORD="your-db-password" \
  --from-literal=JWT_SECRET="your-jwt-secret" \
  --dry-run=client -o yaml > secrets-update.yaml
```

## Scaling Configuration

### Horizontal Pod Autoscaler

The HPA is configured to scale based on:
- CPU utilization (70% threshold)
- Memory utilization (80% threshold)
- Custom metrics (HTTP requests/sec, queue length)
- External metrics (Kafka consumer lag)

```yaml
minReplicas: 3
maxReplicas: 20
metrics:
  - type: Resource
    resource:
      name: cpu
      target:
        type: Utilization
        averageUtilization: 70
```

### Vertical Pod Autoscaler

VPA automatically adjusts resource requests/limits:

```yaml
updatePolicy:
  updateMode: "Auto"
resourcePolicy:
  containerPolicies:
  - containerName: portfolio-accounting
    maxAllowed:
      cpu: 2
      memory: 4Gi
```

## Network Security

### Network Policies

Network policies implement zero-trust security:

1. **Default Deny**: Block all traffic by default
2. **API Server Policy**: Allow ingress from ingress controllers and monitoring
3. **Database Policy**: Allow access only from application pods
4. **Cache Policy**: Allow inter-cluster communication and application access
5. **DNS Policy**: Allow DNS resolution
6. **Monitoring Policy**: Allow Prometheus scraping

### Ingress Configuration

Multiple ingress configurations for different use cases:

1. **Public Ingress**: External access with TLS and rate limiting
2. **Internal Ingress**: Internal access with IP whitelisting
3. **Gateway API**: Modern ingress using Gateway API

## Monitoring and Observability

### Prometheus Integration

The deployment includes:
- ServiceMonitor for automatic service discovery
- PodMonitor for custom metrics collection
- Health check endpoints
- Business metrics exposure

### Health Checks

Three types of health checks:
- **Startup Probe**: Initial application startup (30 attempts)
- **Liveness Probe**: Application health (every 10s)
- **Readiness Probe**: Traffic readiness (every 5s)

### Logging

Structured logging configuration:
- JSON format for machine processing
- Log level: info (configurable)
- Request correlation IDs
- Error tracking and alerting

## Storage Configuration

### PostgreSQL Persistence

- **Storage Class**: Uses default storage class
- **Volume Size**: 20Gi (configurable)
- **Access Mode**: ReadWriteOnce
- **Backup Strategy**: Manual snapshots recommended

### Hazelcast Persistence

- **Volume Size**: 5Gi per node
- **Replication**: 1 backup copy
- **Cluster Size**: 3 nodes minimum

## Deployment Environments

### Production

```bash
# Production deployment
./scripts/k8s-deploy.sh deploy -e production -t v1.0.0 -r 3
```

Configuration:
- High availability (3+ replicas)
- Resource limits enforced
- Network policies enabled
- TLS certificates via cert-manager
- Monitoring and alerting enabled

### Staging

```bash
# Staging deployment
./scripts/k8s-deploy.sh deploy -e staging -t latest -r 2
```

Configuration:
- Reduced resource requirements
- Relaxed network policies
- Self-signed certificates
- Debug logging enabled

### Development

```bash
# Development deployment
./scripts/k8s-deploy.sh deploy -e development -t dev -r 1
```

Configuration:
- Minimal resources
- NodePort services for easy access
- Development database
- Verbose logging

## Operations

### Deployment Commands

```bash
# Deploy new version
./scripts/k8s-deploy.sh upgrade -t v1.2.0

# Rollback to previous version
./scripts/k8s-deploy.sh rollback

# Check deployment status
./scripts/k8s-deploy.sh status

# View logs
./scripts/k8s-deploy.sh logs

# Run tests
./scripts/k8s-deploy.sh test

# Scale manually
kubectl scale deployment portfolio-accounting-service --replicas=5 -n globeco-portfolio-accounting
```

### Database Operations

```bash
# Run migrations
./scripts/k8s-deploy.sh migration

# Access database
kubectl exec -it postgresql-0 -n globeco-portfolio-accounting -- psql -U portfolio_user -d portfolio_accounting

# Backup database
kubectl exec postgresql-0 -n globeco-portfolio-accounting -- pg_dump -U postgres portfolio_accounting > backup.sql
```

### Cache Operations

```bash
# Check Hazelcast cluster
kubectl exec -it hazelcast-0 -n globeco-portfolio-accounting -- hz-cli

# View cache metrics
kubectl port-forward hazelcast-0 8080:8080 -n globeco-portfolio-accounting
curl http://localhost:8080/metrics
```

## Troubleshooting

### Common Issues

1. **Pods not starting**
   ```bash
   kubectl describe pod <pod-name> -n globeco-portfolio-accounting
   kubectl logs <pod-name> -n globeco-portfolio-accounting
   ```

2. **Service not accessible**
   ```bash
   kubectl get svc -n globeco-portfolio-accounting
   kubectl describe svc portfolio-accounting-service -n globeco-portfolio-accounting
   ```

3. **Database connection issues**
   ```bash
   kubectl exec -it <app-pod> -n globeco-portfolio-accounting -- nc -zv postgresql-service 5432
   ```

4. **Cache connection issues**
   ```bash
   kubectl exec -it <app-pod> -n globeco-portfolio-accounting -- telnet hazelcast-service 5701
   ```

### Debug Commands

```bash
# Get all resources
kubectl get all -n globeco-portfolio-accounting

# Check events
kubectl get events -n globeco-portfolio-accounting --sort-by='.lastTimestamp'

# Check network policies
kubectl get networkpolicy -n globeco-portfolio-accounting

# Check resource usage
kubectl top pods -n globeco-portfolio-accounting
kubectl top nodes
```

## Security Considerations

### Production Security Checklist

- [ ] Update default passwords in secrets
- [ ] Configure proper RBAC permissions
- [ ] Enable network policies
- [ ] Use non-root containers
- [ ] Enable read-only root filesystem
- [ ] Configure resource limits
- [ ] Use TLS for all communications
- [ ] Regular security scanning of images
- [ ] Monitor for security events

### Best Practices

1. **Secrets Management**: Use external secret management (Vault, AWS Secrets Manager)
2. **Image Security**: Scan images for vulnerabilities
3. **Network Security**: Implement zero-trust networking
4. **Access Control**: Limit cluster access with RBAC
5. **Monitoring**: Set up security monitoring and alerting

## Performance Tuning

### Application Tuning

- Adjust JVM settings for Hazelcast
- Optimize database connection pooling
- Configure cache TTL based on usage patterns
- Tune HPA metrics and thresholds

### Infrastructure Tuning

- Use appropriate node types for workloads
- Configure anti-affinity for high availability
- Optimize storage performance
- Monitor and adjust resource requests/limits

## Backup and Recovery

### Database Backup

```bash
# Create backup
kubectl exec postgresql-0 -n globeco-portfolio-accounting -- \
  pg_dump -U postgres portfolio_accounting > portfolio_backup_$(date +%Y%m%d).sql

# Restore backup
kubectl exec -i postgresql-0 -n globeco-portfolio-accounting -- \
  psql -U postgres portfolio_accounting < portfolio_backup_20240101.sql
```

### Configuration Backup

```bash
# Backup all configurations
kubectl get all,configmaps,secrets,ingress,networkpolicies \
  -n globeco-portfolio-accounting -o yaml > portfolio_config_backup.yaml
```

## Support

For support and questions:
- **Email**: noah@kasbench.org
- **Documentation**: https://github.com/kasbench/globeco-portfolio-accounting-service
- **Issues**: Create GitHub issues for bugs and feature requests

## License

This deployment configuration is part of the GlobeCo Portfolio Accounting Service project. 