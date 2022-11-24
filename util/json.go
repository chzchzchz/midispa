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

func SaveMapValuesJSONFile[K comparable, V any](path string, m map[K]*V) error {
	f, err := os.OpenFile(path, os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	enc.SetIndent("", "\t")
	for _, v := range m {
		if err := enc.Encode(v); err != nil {
			return err
		}
	}
	return nil
}
