LDFLAGS = -X github.com/jamf/regatta-go/client.Version=$(VERSION)
VERSION ?= $(shell git describe --tags --always --dirty)
CGO_ENABLED ?= 0
REGATTA_PROTO_SRC_DIR = client/internal/proto
REGATTA_PROTO_VERSION = v0.2.1

.PHONY: all
all: getproto test build

.PHONY: test
test:
	go test ./... -cover -race -v

.PHONY: build
build:
	test $(VERSION) || (echo "version not set"; exit 1)
	CGO_ENABLED=$(CGO_ENABLED) go build -ldflags="$(LDFLAGS)" -v ./...



.PHONY: getproto-cleanup
# Cleanup temporary directory
getproto-cleanup:
	rm -Rf ${REGATTA_PROTO_SRC_DIR}

.PHONY: getproto
getproto: getproto-cleanup
	mkdir -p ${REGATTA_PROTO_SRC_DIR}
	curl -sSL https://api.github.com/repos/jamf/regatta/tarball/${REGATTA_PROTO_VERSION} | tar -x --strip-components=2 -C ${REGATTA_PROTO_SRC_DIR} */regattapb/*.pb.go


