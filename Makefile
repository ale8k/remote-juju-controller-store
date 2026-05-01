BUILD_DIR  := build
BINARY_SRV := $(BUILD_DIR)/rcsd
BINARY_RCS := $(BUILD_DIR)/rcs
ADDR       ?= :8484
DB         ?= tmp/store.db

.PHONY: build build-srv build-rcs link serve generate

build-srv:
	mkdir -p $(BUILD_DIR)
	go build -o $(BINARY_SRV) ./cmd/srv

build-rcs:
	mkdir -p $(BUILD_DIR)
	go build -o $(BINARY_RCS) ./cmd/rcs

build: build-srv build-rcs

link: build-rcs
	ln -sf $(BUILD_DIR)/rcs rcs

serve: build-srv
	mkdir -p tmp
	./$(BINARY_SRV) --addr $(ADDR) --db $(DB)

generate:
	mkdir -p pkg/client
	go generate ./api/... ./internal/db/...
	@echo "generated pkg/client/client.gen.go"


