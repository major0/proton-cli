package internal

import (
	"encoding/json"
	"log/slog"
)

/* CopyStruct copies a subset of the source structure into the target
 * structure. The rules for this copy are pretty straight forward. All
 * fields found in the target MUST be also found in the source, and they
 * must also be of the same type as those fields found in the source. This
 * presents a behavior found in C/C++, Nim and other languages. */
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
