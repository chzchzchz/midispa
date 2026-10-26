CMDS:=$(wildcard cmd/*)
TARGETS=$(subst cmd/,out/,$(CMDS))
all: $(TARGETS)

.PHONY: generate
generate:
	go generate ./...
	
out/%: cmd/% generate
	go build -o $@ ./$<