export GOPATH ?= ${HOME}/go
export GO111MODULE=on
GOPROXY ?= https://proxy.golang.org,direct
export GOPROXY

${GOPATH}/bin/tomlv:
	go install github.com/BurntSushi/toml/cmd/tomlv@latest

reqs: ${GOPATH}/bin/tomlv
	go mod download

clean:
	rm -f fusee

test: clean reqs
	${GOPATH}/bin/tomlv configs/config.toml

fusee: test
	go build cmd/fusee/fusee.go

build: fusee
