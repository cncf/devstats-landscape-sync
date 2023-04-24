GO_BIN_FILES=check_sync.go
#for race CGO_ENABLED=1
#GO_ENV=CGO_ENABLED=1
GO_ENV=CGO_ENABLED=0
#GO_BUILD=go build -ldflags '-s -w' -race
GO_BUILD=go build -ldflags '-s -w'
GO_FMT=gofmt -s -w
GO_LINT=golint -set_exit_status
GO_VET=go vet
GO_IMPORTS=goimports -w
GO_USEDEXPORTS=usedexports
GO_ERRCHECK=errcheck -asserts -ignore '[FS]?[Pp]rint*'
BINARIES=check_sync
all: check ${BINARIES}
check_sync: check_sync.go
	 ${GO_ENV} ${GO_BUILD} -o check_sync check_sync.go
fmt: ${GO_BIN_FILES}
	${GO_FMT} ${GO_BIN_FILES}
lint: ${GO_BIN_FILES}
	${GO_LINT} ${GO_BIN_FILES}
vet: ${GO_BIN_FILES}
	${GO_VET} ${GO_BIN_FILES}
imports: ${GO_BIN_FILES}
	${GO_IMPORTS} ${GO_BIN_FILES}
usedexports: ${GO_BIN_FILES}
	${GO_USEDEXPORTS} ./...
errcheck: ${GO_BIN_FILES}
	${GO_ERRCHECK} ./...
check: fmt lint imports vet usedexports errcheck
clean:
	rm -f ${BINARIES}
.PHONY: all
