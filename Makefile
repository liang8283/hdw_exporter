PROJECTNAME=$(shell basename "$(PWD)")

# Make is verbose in Linux. Make it silent.
MAKEFLAGS += --silent

.PHONY: clean
clean:
	@echo " > Cleaning build cache..."
	go clean
	rm -f bin/hdw_exporter
	rm -fr bin/dist

.PHONY: build
build:
	@echo " > Building binary..."
	if [ ! -d bin/ ]; then mkdir bin/ ; fi;
	go mod download && go build -o ./bin/hdw_exporter

.PHONY: package
package:
	@echo " > Archive binary target files and srcipts..."
	if [ ! -d bin/ ]; then mkdir bin/ ; fi;
	cd bin/ && mkdir -p dist && mkdir -p tmp && cd -
	go build -o ./bin/tmp/hdw_exporter
	cd bin/tmp/ && tar -czvf ../dist/hdw_exporter.tar.gz * && cd -
	cd bin/ && rm -fr tmp/ && cd -
