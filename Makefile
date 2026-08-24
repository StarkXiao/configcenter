.PHONY: run fmt test vet check size

run:
	go run ./cmd/server

fmt:
	gofmt -w $$(find . -name '*.go')

test:
	go test ./...

vet:
	go vet ./...

size:
	@files=$$(find . -name '*.go' ! -name '*_test.go' | wc -l | tr -d ' '); \
	lines=$$(find . -name '*.go' ! -name '*_test.go' -print0 | xargs -0 wc -l | tail -1 | awk '{print $$1}'); \
	echo "non-test Go files: $$files"; echo "non-test Go lines: $$lines"; \
	test $$files -gt 20 -a $$files -lt 25; test $$lines -gt 2000 -a $$lines -lt 2200

check: fmt test vet size
