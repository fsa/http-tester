PROGRAM := http-tester
VERSION := $(shell git describe --tags --always 2>/dev/null || echo dev)

.PHONY: build run test vet clean lint

## Сборка dev-версии
build:
	go build -o $(PROGRAM) .

## Запуск (передайте аргументы через ARGS)
#   make run ARGS="tavda.info"
#   make run ARGS="-c lib.tavda.info.yaml"
run: build
	./$(PROGRAM) $(ARGS)

## Тесты
test:
	go test ./...

## Статический анализ
vet:
	go vet ./...

## Линтер (если установлен golangci-lint)
lint:
	golangci-lint run ./...

## Сборка релиза (все платформы)
release:
	./scripts/build-release.sh

## Очистка
clean:
	rm -f $(PROGRAM)
	rm -rf dist/

## Помощь
help:
	@echo "Цели:"
	@echo "  build   — сборка dev-версии"
	@echo "  run     — запуск (make run ARGS=\"tavda.info\")"
	@echo "  test    — тесты"
	@echo "  vet     — статический анализ"
	@echo "  lint    — линтер (golangci-lint)"
	@echo "  release — сборка релиза для всех платформ"
	@echo "  clean   — очистка"
