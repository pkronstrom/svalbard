.PHONY: build build-dev build-drive-runtime test clean

build: build-drive-runtime
	cd host-cli && go build -o ../bin/svalbard ./cmd/svalbard/

build-dev:
	cd host-cli && go build -o ../bin/svalbard ./cmd/svalbard/

build-drive-runtime:
	scripts/build-drive-runtime.sh

test:
	cd host-cli && go test ./...

clean:
	rm -rf bin/svalbard bin/svalbard-drive host-cli/internal/toolkit/embedded/*/
