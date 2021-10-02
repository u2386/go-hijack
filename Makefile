.PHONY: clean testdata build test run-test

DEBUG =
ifdef GOHIJACK_BUILD_DEBUG
	DEBUG = -X github.com/u2386/go-hijack.DEBUG=yes
endif
LDFLAGS = $(DEBUG)

clean:
	@rm -rf ./output \
		./*.out \
		go-hijack.test \
		main

testdata:
	@mkdir -p output
	@(cd test/testdata; go build -gcflags='all=-l -N' -o ../../output/sample)

build:
	@go build -o main

test:
	@ginkgo build -gcflags='-l -N' -cover -ldflags "$(LDFLAGS)"

run-test: clean test
	@./go-hijack.test -ginkgo.v