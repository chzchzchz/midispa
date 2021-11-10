package util

import (
	"os"
	"path/filepath"
	"strings"
)

func Walk(dir string) []string {
	paths := walk(dir)
	for i := range paths {
		paths[i] = strings.TrimPrefix(paths[i], filepath.Clean(dir)+"/")
	}
	return paths
}

func walk(dir string) (ret []string) {
	des, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	for _, de := range des {
		if de.IsDir() {
			ret = append(ret, walk(filepath.Join(dir, de.Name()))...)
		} else {
			ret = append(ret, filepath.Join(dir, de.Name()))
		}
	}
	return ret
}
