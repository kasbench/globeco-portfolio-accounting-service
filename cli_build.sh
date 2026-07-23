docker buildx build --platform linux/amd64,linux/arm64  \
	--target cli \
	-t kasbench/globeco-portfolio-accounting-service-cli:latest \
	-t kasbench/globeco-portfolio-accounting-service-cli:1.0.0 \
	--push .