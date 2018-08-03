docker:
	@docker build -t $(DOCKER_TAG) .
	@docker push $(DOCKER_TAG)