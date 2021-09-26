.PHONY: clean testdata build test

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
	ginkgo build -gcflags='-l -N' -cover