.PHONY: clean testdata build

clean:
	@rm -rf ./output \
		main

testdata:
	@mkdir -p output
	@(cd test/testdata; go build -gcflags='all=-l -N' -o ../../output/sample)

build:
	@go build -o main