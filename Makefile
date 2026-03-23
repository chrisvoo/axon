.PHONY: build test clean release-assets web dev

BINARY=axon

web:
	cd web && pnpm install --frozen-lockfile && pnpm build
	rm -rf internal/dashboard/dist
	mkdir -p internal/dashboard/dist
	cp -R web/dist/. internal/dashboard/dist/

build: web
	go build -o $(BINARY) ./cmd/axon

# Run locally over plain HTTP (no TLS) so Cursor can connect without cert setup.
# Requires axon init to have been run at least once (for API key + denylist).
dev:
	go run ./cmd/axon serve -dev -no-browser

test:
	go test ./...

clean:
	rm -f $(BINARY) axon-*.tar.gz axon-*.zip

# Cross-compile release binaries (output in dist/)
release-assets: web
	mkdir -p dist
	GOOS=linux   GOARCH=amd64 go build -o dist/$(BINARY)-linux-amd64 ./cmd/axon
	GOOS=linux   GOARCH=arm64 go build -o dist/$(BINARY)-linux-arm64 ./cmd/axon
	GOOS=darwin  GOARCH=amd64 go build -o dist/$(BINARY)-darwin-amd64 ./cmd/axon
	GOOS=darwin  GOARCH=arm64 go build -o dist/$(BINARY)-darwin-arm64 ./cmd/axon
	GOOS=windows GOARCH=amd64 go build -o dist/$(BINARY)-windows-amd64.exe ./cmd/axon
	GOOS=windows GOARCH=arm64 go build -o dist/$(BINARY)-windows-arm64.exe ./cmd/axon
	cd dist && cp $(BINARY)-linux-amd64 axon && tar czf ../axon-linux-amd64.tar.gz axon && rm axon
	cd dist && cp $(BINARY)-linux-arm64 axon && tar czf ../axon-linux-arm64.tar.gz axon && rm axon
	cd dist && cp $(BINARY)-darwin-amd64 axon && tar czf ../axon-darwin-amd64.tar.gz axon && rm axon
	cd dist && cp $(BINARY)-darwin-arm64 axon && tar czf ../axon-darwin-arm64.tar.gz axon && rm axon
	cd dist && cp $(BINARY)-windows-amd64.exe axon.exe && zip ../axon-windows-amd64.zip axon.exe && rm axon.exe
	cd dist && cp $(BINARY)-windows-arm64.exe axon.exe && zip ../axon-windows-arm64.zip axon.exe && rm axon.exe
