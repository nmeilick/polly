.PHONY: all build clean

# Determine the current Git commit, origin, and build time
GIT_VERSION := $(shell git describe --abbrev=0 --match="v*" || echo 0.0.0)
GIT_COMMIT := $(shell git rev-parse --short HEAD)
GIT_ORIGIN := $(shell git config --get remote.origin.url)
BUILD_TIME := '$(shell date -u +"%Y-%m-%dT%H:%M:%S")'

# Set the ldflags to pass to the go build command
LDFLAGS := -X 'main.Version=$(GIT_VERSION)' \
           -X 'main.Commit=$(GIT_COMMIT)' \
           -X 'main.Origin=$(GIT_ORIGIN)' \
           -X 'main.BuildTime="$(BUILD_TIME)"'

all: build

build:
	mkdir -p bin
	go build -ldflags "$(LDFLAGS)" -o bin/polly

clean:
	rm -f bin/polly
