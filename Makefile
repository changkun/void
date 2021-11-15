# Copyright (c) 2021 Changkun Ou <hi@changkun.de>. All Rights Reserved.
# Unauthorized using, copying, modifying and distributing, via any
# medium is strictly prohibited.

NAME = void
BUILD_FLAGS = -o $(NAME) --tags prod -mod=vendor

all:
	go build $(BUILD_FLAGS)
serv:
	./$(NAME) serv
initdb:
	cd data && go run initdb.go
build:
	CGO_ENABLED=0 GOOS=linux go build $(BUILD_FLAGS)
	docker build -f docker/Dockerfile -t $(NAME):latest .
up:
	docker-compose -f docker/docker-compose.yml up -d
down:
	docker-compose -f docker/docker-compose.yml down
clean:
	rm -rf $(NAME)
	docker rmi -f $(shell docker images -f "dangling=true" -q) 2> /dev/null; true
	docker rmi -f $(NAME):latest 2> /dev/null; true