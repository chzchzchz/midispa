package util

import (
	"encoding/json"
	"os"
)

func MustLoadJSONFile[T any](path string) []T {
	ret, err := LoadJSONFile[T](path)
	if err != nil {
		panic(err)
	}
	return ret
}

func LoadJSONFile[T any](path string) (cfgs []T, err error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	dec := json.NewDecoder(f)
	for dec.More() {
		var c T
		if err := dec.Decode(&c); err != nil {
			return nil, err
		}
		cfgs = append(cfgs, c)
	}
	return cfgs, nil
}
