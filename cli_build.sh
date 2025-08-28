docker buildx build --platform linux/amd64,linux/arm64  \
	--target cli \
	-t kasbench/globeco-portfolio-accounting-service-cli:latest \
	--push .