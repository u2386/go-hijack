.PHONY: clean testdata build test run-test

DEBUG =
ifdef GOHIJACK_BUILD_DEBUG
	DEBUG = -X github.com/u2386/go-hijack/runtime.DEBUG=yes
endif
LDFLAGS = $(DEBUG)

clean:
	@rm -rf ./output \
		./*.out \
		go-hijack.test \
		main
	@find . -name 'cover.out' -type f -exec rm {} \;

testdata:
	@mkdir -p output
	@(cd test/testdata; go build -gcflags='all=-l -N' -o ../../output/sample)

build:
	@go build -o main

test:
	@mkdir -p ./output/test/cover
	@ginkgo -p -r -gcflags='-l -N' -cover -ldflags "$(LDFLAGS)" -outputdir ./output/test/cover -coverprofile cover.out

build-test: clean test
	@@ginkgo build -r -gcflags='-l -N' -cover -ldflags "$(LDFLAGS)"