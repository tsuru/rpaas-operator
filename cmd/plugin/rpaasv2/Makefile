GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
BINARY_NAME=rpaasv2

all: test build

build:
	$(GOBUILD) -o $(BINARY_NAME) -v -mod=vendor

test:
	$(GOTEST) -mod=vendor -v ./...

clean:
	$(GOCLEAN)
	rm $(BINARY_NAME)
run:
	$(GOBUILD) -o $(BINARY_NAME) -v ./...
	./$(BINARY_NAME)

