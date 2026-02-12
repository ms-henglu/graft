package utils

import "encoding/json"

// DeepCopyValue creates a deep copy of a value to avoid mutation during processing.
func DeepCopyValue(val interface{}) interface{} {
	data, err := json.Marshal(val)
	if err != nil {
		return val
	}
	var result interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		return val
	}
	return result
}
