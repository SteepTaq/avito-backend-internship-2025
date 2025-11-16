.PHONY: build run test clean docker-up docker-down migrate-up migrate-down \
	e2e-up e2e-down e2e-test e2e-test-standalone e2e-logs e2e-clean \
	test-unit test-handler test-repository loadtest

BINARY_NAME=avito-service
DOCKER_COMPOSE=docker-compose

build:
	go build -o bin/$(BINARY_NAME) ./cmd/main.go

run: build
	./bin/$(BINARY_NAME)

test:
	go test -v ./...

test-unit:
	go test -v ./internal/service/...

test-handler:
	go test -v ./internal/handler/...

test-repository:
	go test -v -tags=integration ./internal/repository/...

clean:
	rm -rf bin/

docker-up:
	$(DOCKER_COMPOSE) up -d

docker-down:
	$(DOCKER_COMPOSE) down

docker-down-clean:
	$(DOCKER_COMPOSE) down -v

migrate-up:
	$(DOCKER_COMPOSE) run --rm migrate -command=up -migrations-path=./migrations

migrate-down:
	$(DOCKER_COMPOSE) run --rm migrate -command=down -migrations-path=./migrations

migrate-version:
	$(DOCKER_COMPOSE) run --rm migrate -command=version -migrations-path=./migrations

migrate-up-local:
	go run ./cmd/migrate -command=up -migrations-path=./migrations

migrate-down-local:
	go run ./cmd/migrate -command=down -migrations-path=./migrations

logs:
	$(DOCKER_COMPOSE) logs -f app

deps:
	go mod download
	go mod tidy

fmt:
	go fmt ./...
	goimports -w .

lint:
	golangci-lint run ./...

rebuild: docker-down-clean docker-up

generate-mocks:
	mockery

E2E_COMPOSE=docker-compose -f docker-compose.e2e.yaml --project-name avito-e2e
E2E_DB_URL=postgres://postgres:postgres@localhost:5433/test_db?sslmode=disable
E2E_APP_URL=http://localhost:8081

e2e-up:
	$(E2E_COMPOSE) up -d
	@echo "Waiting for services to be ready..."
	@timeout=60; \
	while [ $$timeout -gt 0 ]; do \
		if docker exec avito_e2e_postgres pg_isready -U postgres > /dev/null 2>&1; then \
			if docker exec avito_e2e_postgres psql -U postgres -lqt | cut -d \| -f 1 | grep -qw test_db; then \
				if curl -s http://localhost:8081/health > /dev/null 2>&1; then \
					echo "All services are ready!"; \
					break; \
				fi; \
			fi; \
		fi; \
		sleep 1; \
		timeout=$$((timeout-1)); \
	done; \
	if [ $$timeout -eq 0 ]; then \
		echo "Services failed to start"; \
		$(E2E_COMPOSE) logs; \
		exit 1; \
	fi
	@echo "E2E environment is ready"

e2e-down:
	$(E2E_COMPOSE) down

e2e-down-clean:
	$(E2E_COMPOSE) down -v

e2e-logs:
	$(E2E_COMPOSE) logs -f

e2e-test: e2e-up
	@echo "Running E2E tests with Docker application..."
	E2E_APP_URL="$(E2E_APP_URL)" TEST_DATABASE_URL="$(E2E_DB_URL)" go test -v -tags=e2e ./e2e/...
	@$(MAKE) e2e-down

e2e-test-standalone:
	@echo "Running E2E tests with external services..."
	E2E_APP_URL="${E2E_APP_URL:-http://localhost:8081}" TEST_DATABASE_URL="${E2E_DB_URL}" go test -v -tags=e2e ./e2e/...

# Нагрузочное тестирование
LOADTEST_URL ?= http://localhost:8080

loadtest:
	@echo "Running load tests against $(LOADTEST_URL)..."
	@go run ./cmd/loadtest -url $(LOADTEST_URL)