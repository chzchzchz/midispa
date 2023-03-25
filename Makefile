CMDS:=$(wildcard cmd/*)
TARGETS=$(subst cmd/,out/,$(CMDS))
all: $(TARGETS)

out/%: cmd/%
	go build -o $@ ./$<