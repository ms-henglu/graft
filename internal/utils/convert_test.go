package utils

import (
	"testing"

	"github.com/zclconf/go-cty/cty"
)

func TestInterfaceToCtyValueConversions(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected cty.Value
	}{
		{
			name:     "nil",
			input:    nil,
			expected: cty.NullVal(cty.DynamicPseudoType),
		},
		{
			name:     "bool true",
			input:    true,
			expected: cty.True,
		},
		{
			name:     "bool false",
			input:    false,
			expected: cty.False,
		},
		{
			name:     "integer",
			input:    float64(42),
			expected: cty.NumberIntVal(42),
		},
		{
			name:     "zero",
			input:    float64(0),
			expected: cty.NumberIntVal(0),
		},
		{
			name:     "negative integer",
			input:    float64(-5),
			expected: cty.NumberIntVal(-5),
		},
		{
			name:     "float",
			input:    3.14,
			expected: cty.NumberFloatVal(3.14),
		},
		{
			name:     "string",
			input:    "hello",
			expected: cty.StringVal("hello"),
		},
		{
			name:     "empty string",
			input:    "",
			expected: cty.StringVal(""),
		},
		{
			name:     "empty slice",
			input:    []interface{}{},
			expected: cty.ListValEmpty(cty.DynamicPseudoType),
		},
		{
			name:  "slice of strings",
			input: []interface{}{"a", "b", "c"},
			expected: cty.TupleVal([]cty.Value{
				cty.StringVal("a"),
				cty.StringVal("b"),
				cty.StringVal("c"),
			}),
		},
		{
			name:  "mixed slice",
			input: []interface{}{"text", float64(1), true},
			expected: cty.TupleVal([]cty.Value{
				cty.StringVal("text"),
				cty.NumberIntVal(1),
				cty.True,
			}),
		},
		{
			name:  "slice with nil",
			input: []interface{}{"a", nil, "c"},
			expected: cty.TupleVal([]cty.Value{
				cty.StringVal("a"),
				cty.NullVal(cty.DynamicPseudoType),
				cty.StringVal("c"),
			}),
		},
		{
			name:     "empty map",
			input:    map[string]interface{}{},
			expected: cty.MapValEmpty(cty.DynamicPseudoType),
		},
		{
			name: "map",
			input: map[string]interface{}{
				"name": "test",
				"port": float64(8080),
			},
			expected: cty.ObjectVal(map[string]cty.Value{
				"name": cty.StringVal("test"),
				"port": cty.NumberIntVal(8080),
			}),
		},
		{
			name: "nested map",
			input: map[string]interface{}{
				"outer": map[string]interface{}{
					"inner": "deep",
				},
			},
			expected: cty.ObjectVal(map[string]cty.Value{
				"outer": cty.ObjectVal(map[string]cty.Value{
					"inner": cty.StringVal("deep"),
				}),
			}),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ToCtyValue(tt.input)
			if !got.RawEquals(tt.expected) {
				t.Errorf("interfaceToCtyValue(%v) = %#v, want %#v", tt.input, got, tt.expected)
			}
		})
	}
}
