kubectl apply -f portolio-accounting-service-postgresql-deployment.yaml

kubectl apply -f portolio-accounting-service-redis.yaml

echo "Waiting for PostgreSQL and Hazelcast StatefulSets to be ready..."

kubectl wait --for=condition=ready pod -l app=globeco-portfolio-accounting-service-postgresql -n globeco --timeout=300s
if [ $? -ne 0 ]; then
    echo "Error: PostgreSQL StatefulSet pods did not become ready within timeout"
    exit 1
fi

kubectl wait --for=condition=ready pod -l app=globeco-portfolio-accounting-service-redis -n globeco --timeout=300s
if [ $? -ne 0 ]; then
    echo "Error: Redis StatefulSet pods did not become ready within timeout" 
    exit 1
fi

echo "PostgreSQL and Redis StatefulSets are ready"


kubectl apply -f portolio-accounting-service-server.yaml


