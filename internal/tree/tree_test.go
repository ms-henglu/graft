package tree

import (
	"bytes"
	"strings"
	"testing"
)

func TestFprint(t *testing.T) {
	tests := []struct {
		name     string
		root     *Node
		expected string
	}{
		{
			name:     "single node",
			root:     &Node{Name: "root"},
			expected: "root\n",
		},
		{
			name: "simple children",
			root: &Node{
				Name: "root",
				Children: []*Node{
					{Name: "child1"},
					{Name: "child2"},
				},
			},
			expected: strings.TrimLeft(`
root
├── child1
└── child2
`, "\n"),
		},
		{
			name: "nested children",
			root: &Node{
				Name: "root",
				Children: []*Node{
					{
						Name: "child1",
						Children: []*Node{
							{Name: "grandchild1"},
						},
					},
					{
						Name: "child2",
					},
				},
			},
			expected: strings.TrimLeft(`
root
├── child1
│   └── grandchild1
└── child2
`, "\n"),
		},
		{
			name: "deep nested",
			root: &Node{
				Name: "root",
				Children: []*Node{
					{
						Name: "child1",
						Children: []*Node{
							{
								Name: "grandchild1",
								Children: []*Node{
									{Name: "greatgrandchild1"},
								},
							},
						},
					},
				},
			},
			expected: strings.TrimLeft(`
root
└── child1
    └── grandchild1
        └── greatgrandchild1
`, "\n"),
		},
		{
			name: "complex tree",
			root: &Node{
				Name: "root",
				Children: []*Node{
					{
						Name: "child1",
						Children: []*Node{
							{Name: "grandchild1"},
							{Name: "grandchild2"},
						},
					},
					{
						Name: "child2",
						Children: []*Node{
							{Name: "grandchild3"},
						},
					},
					{
						Name: "child3",
					},
				},
			},
			expected: strings.TrimLeft(`
root
├── child1
│   ├── grandchild1
│   └── grandchild2
├── child2
│   └── grandchild3
└── child3
`, "\n"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			Fprint(&buf, tt.root)
			if got := buf.String(); got != tt.expected {
				t.Errorf("Fprint() = \n%v, want \n%v", got, tt.expected)
			}
		})
	}
}
