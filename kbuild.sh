kubectl delete -f k8s/portfolio-accounting-service-server.yaml
docker buildx build --platform linux/amd64,linux/arm64  \
	--target production \
	-t kasbench/globeco-portfolio-accounting-service-server:latest \
	--push .
kubectl apply -f k8s/portfolio-accounting-service-server.yaml
