docker buildx build --platform linux/amd64,linux/arm64  \
	--target production \
	-t kasbench/globeco-portfolio-accounting-service-server:latest \
	-t kasbench/globeco-portfolio-accounting-service-server:1.0.1 \
	--push .
kubectl delete -f k8s/portfolio-accounting-service-server.yaml
kubectl apply -f k8s/portfolio-accounting-service-server.yaml
