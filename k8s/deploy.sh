kubectl apply -f portolio-accounting-service-postgresql-deployment.yaml

kubectl apply -f portolio-accounting-service-hazelcast.yaml

echo "Waiting for PostgreSQL and Hazelcast StatefulSets to be ready..."

kubectl wait --for=condition=ready pod -l app=globeco-portfolio-accounting-service-postgresql -n globeco --timeout=300s
if [ $? -ne 0 ]; then
    echo "Error: PostgreSQL StatefulSet pods did not become ready within timeout"
    exit 1
fi

kubectl wait --for=condition=ready pod -l app=globeco-portfolio-accounting-service-hazelcast -n globeco --timeout=300s
if [ $? -ne 0 ]; then
    echo "Error: Hazelcast StatefulSet pods did not become ready within timeout" 
    exit 1
fi

echo "PostgreSQL and Hazelcast StatefulSets are ready"


kubectl apply -f portolio-accounting-service-server.yaml


