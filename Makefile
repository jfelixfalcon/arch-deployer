BINARY_NAME=arch_deployer
.DEFAULT_GOAL := build

build:
	cd installer && $(MAKE) build
	cd deployer && $(MAKE) build

update:
	cd installer && $(MAKE) update
	cd deployer && $(MAKE) update

dep:
	cd installer && $(MAKE) dep
	cd deployer && $(MAKE) dep

lint:
	cd installer && $(MAKE) lint
	cd deployer && $(MAKE) lint

clean:
	rm target/*
	cd installer && $(MAKE) clean
	cd deployer && $(MAKE) clean