package utils

import (
	"encoding/json"
	"fmt"

	"github.com/zclconf/go-cty/cty"
	ctyjson "github.com/zclconf/go-cty/cty/json"
)

func ToCtyValue(v interface{}) cty.Value {
	if v == nil {
		return cty.NullVal(cty.DynamicPseudoType)
	}

	switch val := v.(type) {
	case bool:
		return cty.BoolVal(val)
	case float64:
		if val == float64(int64(val)) {
			return cty.NumberIntVal(int64(val))
		}
		return cty.NumberFloatVal(val)
	case string:
		return cty.StringVal(val)
	case []interface{}:
		if len(val) == 0 {
			return cty.ListValEmpty(cty.DynamicPseudoType)
		}
		var vals []cty.Value
		for _, item := range val {
			vals = append(vals, ToCtyValue(item))
		}
		return cty.TupleVal(vals)
	case map[string]interface{}:
		if len(val) == 0 {
			return cty.MapValEmpty(cty.DynamicPseudoType)
		}
		vals := make(map[string]cty.Value)
		for k, item := range val {
			vals[k] = ToCtyValue(item)
		}
		return cty.ObjectVal(vals)
	default:
		jsonBytes, err := json.Marshal(v)
		if err != nil {
			return cty.StringVal(fmt.Sprintf("%v", v))
		}
		ctyVal, err := ctyjson.Unmarshal(jsonBytes, cty.DynamicPseudoType)
		if err != nil {
			return cty.StringVal(string(jsonBytes))
		}
		return ctyVal
	}
}
