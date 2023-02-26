noop:

gostaticcheck:
	@echo "[staticcheck] running honnef.co/go/tools/cmd/staticcheck "
	@staticcheck ./...

govet:
	@echo "[go vet] running go vet"
	@go vet `go list ./... | grep -v quicksilver/proto`

COVERAGE_DIR ?= .coverage
.PHONY: test
gotests:
	@echo "[go test] running unit tests and collecting coverage metrics"
	@mkdir -p $(COVERAGE_DIR)
	@go test -v -race -covermode atomic -coverprofile $(COVERAGE_DIR)/combined.txt `go list ./... | grep -v quicksilver/proto`

extdep:
	@go install honnef.co/go/tools/cmd/staticcheck@latest


dockertest: govet gotests gostaticcheck

test:
	docker build -t github.com/ankur-anand/quicksilver:localtest -f Dockerfile.gotest .
	docker run github.com/ankur-anand/quicksilver:localtest