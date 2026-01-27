package tree

import (
	"fmt"
	"io"
	"os"
)

type Node struct {
	Name     string
	Children []*Node
}

func Print(node *Node) {
	Fprint(os.Stdout, node)
}

func Fprint(w io.Writer, node *Node) {
	_, _ = fmt.Fprintln(w, node.Name)
	printChildren(w, node, "")
}

func printChildren(w io.Writer, node *Node, prefix string) {
	for i, child := range node.Children {
		isLast := i == len(node.Children)-1
		printNode(w, child, prefix, isLast)
	}
}

func printNode(w io.Writer, node *Node, prefix string, isLast bool) {
	connector := "├── "
	if isLast {
		connector = "└── "
	}

	_, _ = fmt.Fprintf(w, "%s%s%s\n", prefix, connector, node.Name)

	childPrefix := prefix + "│   "
	if isLast {
		childPrefix = prefix + "    "
	}

	printChildren(w, node, childPrefix)
}
