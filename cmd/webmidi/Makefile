all: wasm_exec.js main.wasm main

wasm_exec.js: $(shell go env GOROOT)/misc/wasm/wasm_exec.js
	cp $^ $@

main.wasm: $(wildcard *.go)
	GOOS=js GOARCH=wasm go build -o main.wasm

main: $(wildcard *.go)
	go build -o main

.PHONY: clean
clean:
	rm main main.wasm wasm_exec.js
