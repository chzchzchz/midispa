package util

import (
	"encoding/json"
	"os"
)

func MustLoadJSONFile[T any](path string) (cfgs []T) {
	f, err := os.Open(path)
	if err != nil {
		panic(err)
	}
	defer f.Close()
	dec := json.NewDecoder(f)
	for dec.More() {
		var c T
		if err := dec.Decode(&c); err != nil {
			panic(err)
		}
		cfgs = append(cfgs, c)
	}
	return cfgs
}
