.PHONY: build build-dev build-drive-runtime test verify catalog-check clean

build: build-drive-runtime
	cd host-cli && go build -o ../bin/svalbard ./cmd/svalbard/

build-dev:
	cd host-cli && go build -o ../bin/svalbard ./cmd/svalbard/

build-drive-runtime:
	scripts/build-drive-runtime.sh

test: verify

catalog-check:
	scripts/sync-catalog.sh --check

verify: catalog-check
	cd tui && go test ./...
	cd host-tui && go test ./...
	cd host-cli && go test ./...
	cd drive-runtime && go test ./...
	cd build-tools && go test ./...

clean:
	rm -rf bin/svalbard bin/svalbard-drive host-cli/internal/toolkit/embedded/*/
