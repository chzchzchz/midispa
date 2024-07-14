package main

import (
	"flag"
	"fmt"
	"os"
)

func main() {
	flag.Parse()
	f := flag.Args()
	if len(f) != 1 {
		panic("expected file input")
	}
	input, err := os.ReadFile(f[0])
	if err != nil {
		panic(err)
	}
	g := NewGrammar(string(input))
	g.Init()
	if err := g.Parse(); err != nil {
		panic(err)
	}
	//g.PrintSyntaxTree()
	g.Execute()
	fmt.Println("parsing OK: ", g.script.Duration())

	output, err := os.OpenFile("out.mid", os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		panic(err)
	}
	defer output.Close()
	if err := g.script.WriteSMF(output); err != nil {
		panic(err)
	}
}
