BIN := bin/streamdeck-go
ADMIN_BIN := bin/streamdeck-admin
SRC := ./src

.PHONY: build test run install-user restart deploy clean

build:
	cd $(SRC) && go build -o ../$(BIN) ./cmd/streamdeck-go
	cd $(SRC) && go build -o ../$(ADMIN_BIN) ./cmd/streamdeck-admin

test:
	cd $(SRC) && go test ./...

run: build
	./$(BIN)

install-user:
	./scripts/install-user.sh

restart:
	systemctl --user restart streamdeck-go.service

deploy:
	./setup.sh --skip-icons --no-start
	systemctl --user restart streamdeck-go.service

clean:
	rm -rf bin dist
