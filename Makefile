all:
	CGO_ENABLED=0 go build .

install:
	CGO_ENABLED=0 go install .

build-osx:
	GOOS=darwin GOARCH=amd64 CGO_ENABLED=0 go build -o dksnap-osx .

build-linux:
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o dksnap-linux .
