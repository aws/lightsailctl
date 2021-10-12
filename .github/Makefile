app := lightsailctl
workdir := $(shell pwd)
module := $(workdir)
bin := $(workdir)/bin
local_exe := $(bin)/$(app)
sources := $(shell find $(module) -type f -name "*.go" -or -name go.mod -or -name go.sum)

version = $(shell $(local_exe) --version)

# Note that flags "-s -w" disable DWARF and symbol table generation
# to reduce binary size.
build = cd $(module) && \
	env CGO_ENABLED=0 GOOS=$(2) GOARCH=$(3) $(1) build -ldflags "-s -w" \
	-o $(bin)/$(call version)/$(2)-$(3)/$(app)$(4) ./main.go

.PHONY: local test xcompile

local: $(local_exe)

$(local_exe): $(sources) $(lastword $(MAKEFILE_LIST))
	cd $(module) && env GOBIN=$(bin) go install ./...

test:
	cd $(module) && go test ./...

xcompile: local test
	@echo "$(app) version: $(call version)"
	$(call build,go,linux,amd64,)
	$(call build,go,linux,arm64,)
	$(call build,go,darwin,amd64,)
	$(call build,go,darwin,arm64,)
	$(call build,go,windows,amd64,.exe)
	$(call build,go,windows,arm64,.exe)
