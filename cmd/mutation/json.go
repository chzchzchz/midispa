package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/chzchzchz/midispa/cc"
)

// modelJSON rebuilds the selected model and applies the patch gene values before
// marshaling it. The JSON shape therefore comes from the model definition
// rather than a mutation-specific document format.
func (patch *Patch) modelJSON() ([]byte, error) {
	params, err := cc.NewModelParams(patch.model)
	if err != nil {
		return nil, err
	}
	fields, err := cc.ControlFields(params)
	if err != nil {
		return nil, err
	}
	for _, gene := range patch.genes {
		matched := false
		for _, field := range fields {
			if field.Name == gene.name {
				*field.Value = gene.value
				matched = true
				break
			}
		}
		if !matched {
			return nil, fmt.Errorf("model %q has no field %q", patch.model, gene.name)
		}
	}
	return json.Marshal(params)
}

func writePatchJSON(path string, patch *Patch) error {
	data, err := patch.modelJSON()
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o600)
}
