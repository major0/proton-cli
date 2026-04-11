// Package internal provides shared utilities for the proton-cli application.
package internal

import (
	"encoding/json"
	"log/slog"
)

// CopySubStruct copies a subset of the source structure into the target
// structure. All fields found in the target must also be found in the source,
// and they must be of the same type.
func CopySubStruct(source any, destination any) error {
	slog.Debug("CopySubStruct")

	data, err := json.Marshal(source)
	if err != nil {
		return err
	}

	err = json.Unmarshal(data, destination)
	if err != nil {
		return err
	}

	return nil
}
