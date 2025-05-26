.PHONY: run
COMPOSE_FILE=deployments/docker-compose.yaml

run:
	docker compose -f ${COMPOSE_FILE} up --build --remove-orphans

generate:
	protoc pkg/pb/*.proto --proto_path=. \
         --go_out=pkg/pb/ --go_opt=module=github.com/AndreyChufelin/movies-auth/pkg/pb \
         --go-grpc_out=pkg/pb/ --go-grpc_opt=module=github.com/AndreyChufelin/movies-auth/pkg/pb
	go generate ./...