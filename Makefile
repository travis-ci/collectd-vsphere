ROOT_PACKAGE := github.com/travis-ci/collectd-vsphere
MAIN_PACKAGE := $(ROOT_PACKAGE)/cmd/collectd-vsphere

VERSION_VAR := main.VersionString
VERSION_VALUE ?= $(shell git describe --always --dirty --tags 2>/dev/null)
REV_VAR := main.RevisionString
REV_VALUE ?= $(shell git rev-parse HEAD 2>/dev/null || echo "???")
REV_URL_VAR := main.RevisionURLString
REV_URL_VALUE ?= https://github.com/travis-ci/collectd-vsphere/tree/$(shell git rev-parse HEAD 2>/dev/null || echo "'???'")
GENERATED_VAR := main.GeneratedString
GENERATED_VALUE ?= $(shell date -u +'%Y-%m-%dT%H:%M:%S%z')
COPYRIGHT_VAR := main.CopyrightString
COPYRIGHT_VALUE ?= $(shell grep -i ^copyright LICENSE | sed 's/^[Cc]opyright //')

GOPATH := $(shell echo $${GOPATH%%:*})
GOBUILD_LDFLAGS ?= \
    -X '$(VERSION_VAR)=$(VERSION_VALUE)' \
    -X '$(REV_VAR)=$(REV_VALUE)' \
    -X '$(REV_URL_VAR)=$(REV_URL_VALUE)' \
    -X '$(GENERATED_VAR)=$(GENERATED_VALUE)' \
    -X '$(COPYRIGHT_VAR)=$(COPYRIGHT_VALUE)'

.PHONY: all
all: clean test build

.PHONY: clean
clean:
	$(RM) $(GOPATH)/bin/collectd-vsphere
	$(RM) -rv ./build
	find $(GOPATH)/pkg -wholename "*$(ROOT_PACKAGE)*.a" -delete

.PHONY: test
test: deps
	go test -x -v -cover \
		-coverpkg $(ROOT_PACKAGE) \
		-coverprofile package.coverprofile \
		$(ROOT_PACKAGE)

.PHONY: build
build: deps
	go install -x -ldflags "$(GOBUILD_LDFLAGS)" $(MAIN_PACKAGE)

.PHONY: crossbuild
crossbuild: deps
	GOARCH=amd64 GOOS=darwin go build -o build/darwin/amd64/collectd-vsphere \
		-ldflags "$(GOBUILD_LDFLAGS)" $(MAIN_PACKAGE)
	GOARCH=amd64 GOOS=linux go build -o build/linux/amd64/collectd-vsphere \
		-ldflags "$(GOBUILD_LDFLAGS)" $(MAIN_PACKAGE)

.PHONY: distclean
distclean:
	$(RM) -r vendor/

.PHONY: deps
deps: vendor

.PHONY: prereqs
prereqs:
	curl https://glide.sh/get | sh

.PHONY: copyright
copyright:
	sed -i "s/^Copyright.*Travis CI/Copyright © $(shell date +%Y) Travis CI/" LICENSE

vendor:
	glide install