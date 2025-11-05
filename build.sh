docker buildx build --platform linux/amd64,linux/arm64  \
	--target production \
	-t kasbench/globeco-portfolio-accounting-service-server:latest \
	-t kasbench/globeco-portfolio-accounting-service-server:1.0.0 \
	--push .