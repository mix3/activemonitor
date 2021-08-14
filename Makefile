GIT_VER := $(shell git describe --tags)

packages:
	$(MAKE) clean
	GOOS=darwin GOARCH=amd64 go build -o ./dist/activemonitor

release:
	ghr -u mix3 $(GIT_VER) dist/

clean:
	rm -rf dist/
