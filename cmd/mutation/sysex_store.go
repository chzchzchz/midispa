package main

import (
	"fmt"
	"os"
)

// sysexPatchStore writes the encoded vendor messages as a raw SysEx file,
// conventionally ending in .syx. It mirrors the SMF store's numbered history so
// both formats behave the same way across generations.
type sysexPatchStore struct {
	outputPath string
}

func (store sysexPatchStore) path() string {
	return store.outputPath
}

func (store sysexPatchStore) save(target patch, generation int) error {
	candidate, err := asSysexPatch(target)
	if err != nil {
		return err
	}
	generationPath, latestPath, err := generationPaths(store.outputPath, generation)
	if err != nil {
		return err
	}
	messages, err := candidate.encode(0)
	if err != nil {
		return err
	}
	// A file is only worth writing once the whole message is encoded, so a
	// rejected value never leaves a truncated dump behind.
	var encoded []byte
	for _, message := range messages {
		encoded = append(encoded, message...)
	}
	if err := os.WriteFile(generationPath, encoded, 0o600); err != nil {
		return fmt.Errorf("write generation: %w", err)
	}
	if err := os.WriteFile(latestPath, encoded, 0o600); err != nil {
		return fmt.Errorf("write latest output: %w", err)
	}
	return nil
}
