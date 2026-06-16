package dom

import (
	"strings"
)

// A simple CSS selector engine for our custom DOM nodes.
// Supports:
// - Combinators: ' ' (descendant), '>' (child), '+' (adjacent sibling), '~' (general sibling)
// - Simple selectors: element, #id, .class
// - Attribute selectors: [attr], [attr=val], [attr^=val], [attr$=val], [attr*=val]
// - Pseudo-classes: :first-child, :last-child, :nth-child, :not(simple-selector)

type selectorToken struct {
	combinator string // "", " ", ">", "+", "~"
	tag        string
	id         string
	classes    []string
	attrs      []attrSelector
	pseudos    []string
}

type attrSelector struct {
	name  string
	op    string // "", "=", "^=", "$=", "*="
	value string
}

func parseSelector(sel string) []selectorToken {
	// A naive tokenizer for CSS selectors.
	// We'll split by combinators and then parse simple selectors.
	// For simplicity, we handle commas by splitting before parsing.

	// Replace combinators with padded spaces so we can tokenize easily
	// taking care of strings inside brackets
	var tokens []selectorToken

	sel = strings.TrimSpace(sel)
	if sel == "" {
		return tokens
	}

	// This is a simplified regex-based parser
	// It assumes well-formed selectors without complex nested spaces (like inside :not(.a .b))

	// 1. Tokenize into combinators and simple selectors
	var parts []string
	var current strings.Builder
	inAttr := false
	inString := false
	var stringChar rune

	for _, ch := range sel {
		if inString {
			current.WriteRune(ch)
			if ch == stringChar {
				inString = false
			}
			continue
		}
		if inAttr {
			current.WriteRune(ch)
			if ch == ']' {
				inAttr = false
			} else if ch == '"' || ch == '\'' {
				inString = true
				stringChar = ch
			}
			continue
		}

		if ch == '[' {
			inAttr = true
			current.WriteRune(ch)
			continue
		}

		if ch == ' ' || ch == '>' || ch == '+' || ch == '~' {
			if current.Len() > 0 {
				parts = append(parts, current.String())
				current.Reset()
			}
			if ch != ' ' || (len(parts) > 0 && parts[len(parts)-1] != " " && parts[len(parts)-1] != ">" && parts[len(parts)-1] != "+" && parts[len(parts)-1] != "~") {
				// Only add space combinator if we don't already have a combinator
				parts = append(parts, string(ch))
			}
		} else {
			current.WriteRune(ch)
		}
	}
	if current.Len() > 0 {
		parts = append(parts, current.String())
	}

	// Clean up consecutive spaces or combinator + space
	var cleanedParts []string
	for i := 0; i < len(parts); i++ {
		p := strings.TrimSpace(parts[i])
		if p == "" {
			// It was a space combinator
			if i > 0 && i < len(parts)-1 {
				prev := cleanedParts[len(cleanedParts)-1]
				if prev != ">" && prev != "+" && prev != "~" && prev != " " {
					cleanedParts = append(cleanedParts, " ")
				}
			}
		} else if p == ">" || p == "+" || p == "~" {
			if len(cleanedParts) > 0 && cleanedParts[len(cleanedParts)-1] == " " {
				cleanedParts[len(cleanedParts)-1] = p
			} else {
				cleanedParts = append(cleanedParts, p)
			}
		} else {
			cleanedParts = append(cleanedParts, p)
		}
	}

	// Parse simple selectors
	var tok selectorToken
	for i := 0; i < len(cleanedParts); i++ {
		p := cleanedParts[i]
		if p == " " || p == ">" || p == "+" || p == "~" {
			tok.combinator = p
			continue
		}

		t := parseSimpleSelector(p)
		t.combinator = tok.combinator
		tokens = append(tokens, t)
		tok = selectorToken{} // reset for next
	}

	return tokens
}

func parseSimpleSelector(sel string) selectorToken {
	var tok selectorToken
	tok.tag = "*"

	// A simpler manual parser is better to handle multiple classes/attrs
	var current strings.Builder
	mode := 't' // t: tag, #: id, .: class, [: attr, :: pseudo

	inString := false
	var stringChar rune

	finishCurrent := func() {
		val := current.String()
		current.Reset()
		if val == "" {
			return
		}
		switch mode {
		case 't':
			tok.tag = strings.ToUpper(val)
		case '#':
			tok.id = val
		case '.':
			tok.classes = append(tok.classes, val)
		case '[':
			// parse attr
			val = strings.TrimSuffix(val, "]")
			idx := strings.IndexAny(val, "^$*~|=")
			if idx == -1 {
				tok.attrs = append(tok.attrs, attrSelector{name: val})
			} else {
				opLen := 1
				if val[idx] != '=' {
					opLen = 2
				}
				op := val[idx : idx+opLen]
				name := val[:idx]
				v := val[idx+opLen:]
				v = strings.Trim(v, "\"'")
				tok.attrs = append(tok.attrs, attrSelector{name: name, op: op, value: v})
			}
		case ':':
			tok.pseudos = append(tok.pseudos, val)
		}
	}

	for _, ch := range sel {
		if inString {
			current.WriteRune(ch)
			if ch == stringChar {
				inString = false
			}
			continue
		}

		if mode == '[' && ch != ']' {
			if ch == '"' || ch == '\'' {
				inString = true
				stringChar = ch
			}
			current.WriteRune(ch)
			continue
		}
		if mode == ':' && ch != ')' && strings.ContainsRune(current.String(), '(') {
			current.WriteRune(ch)
			continue
		}

		if ch == '#' || ch == '.' || ch == '[' || ch == ':' {
			if mode == '[' && ch == ']' {
				// handled below
			} else {
				finishCurrent()
				mode = ch
				continue
			}
		}

		if ch == ']' && mode == '[' {
			finishCurrent()
			mode = ' '
			continue
		}

		if ch == ')' && mode == ':' {
			current.WriteRune(ch)
			finishCurrent()
			mode = ' '
			continue
		}

		current.WriteRune(ch)
	}
	finishCurrent()

	return tok
}

func matchesToken(node *Node, tok selectorToken) bool {
	if node.NodeType != ElementNode {
		return false
	}

	if tok.tag != "" && tok.tag != "*" {
		if node.NodeName != tok.tag {
			return false
		}
	}

	if tok.id != "" {
		if node.Id != tok.id {
			return false
		}
	}

	for _, cls := range tok.classes {
		found := false
		for _, c := range strings.Fields(node.ClassName) {
			if c == cls {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	for _, attr := range tok.attrs {
		if !node.HasAttribute(attr.name) {
			return false
		}
		val := node.GetAttribute(attr.name)
		switch attr.op {
		case "=":
			if val != attr.value {
				return false
			}
		case "^=":
			if !strings.HasPrefix(val, attr.value) {
				return false
			}
		case "$=":
			if !strings.HasSuffix(val, attr.value) {
				return false
			}
		case "*=":
			if !strings.Contains(val, attr.value) {
				return false
			}
		}
	}

	for _, pseudo := range tok.pseudos {
		if pseudo == "first-child" {
			if node.ParentNode != nil {
				isFirst := true
				for _, sib := range node.ParentNode.ChildNodes {
					if sib.NodeType == ElementNode {
						if sib != node {
							isFirst = false
						}
						break
					}
				}
				if !isFirst {
					return false
				}
			}
		} else if pseudo == "last-child" {
			if node.ParentNode != nil {
				isLast := true
				for i := len(node.ParentNode.ChildNodes) - 1; i >= 0; i-- {
					sib := node.ParentNode.ChildNodes[i]
					if sib.NodeType == ElementNode {
						if sib != node {
							isLast = false
						}
						break
					}
				}
				if !isLast {
					return false
				}
			}
		} else if strings.HasPrefix(pseudo, "not(") && strings.HasSuffix(pseudo, ")") {
			inner := pseudo[4 : len(pseudo)-1]
			innerTok := parseSimpleSelector(inner)
			if matchesToken(node, innerTok) {
				return false
			}
		}
	}

	return true
}

func querySelectorAllImpl(root *Node, selector string) []*Node {
	selector = strings.TrimSpace(selector)
	if selector == "" {
		return nil
	}

	if strings.Contains(selector, ",") {
		var results []*Node
		seen := make(map[int]bool)
		for _, part := range strings.Split(selector, ",") {
			for _, res := range querySelectorAllImpl(root, part) {
				if !seen[res.Uid] {
					seen[res.Uid] = true
					results = append(results, res)
				}
			}
		}
		return results
	}

	tokens := parseSelector(selector)
	if len(tokens) == 0 {
		return nil
	}

	// We start with all descendant elements of root
	var currentNodes []*Node
	var collectAll func(n *Node)
	collectAll = func(n *Node) {
		if n.NodeType == ElementNode {
			currentNodes = append(currentNodes, n)
		}
		for _, c := range n.ChildNodes {
			collectAll(c)
		}
	}
	for _, c := range root.ChildNodes {
		collectAll(c)
	}

	// For each token, we filter the currentNodes or traverse
	for i, tok := range tokens {
		var nextNodes []*Node

		if i == 0 || tok.combinator == " " {
			// ... existing code ...
			if i > 0 {
				var descendants []*Node
				seen := make(map[int]bool)
				for _, n := range currentNodes {
					var collect func(cn *Node)
					collect = func(cn *Node) {
						if cn.NodeType == ElementNode {
							if !seen[cn.Uid] {
								seen[cn.Uid] = true
								descendants = append(descendants, cn)
							}
						}
						for _, cc := range cn.ChildNodes {
							collect(cc)
						}
					}
					for _, c := range n.ChildNodes {
						collect(c)
					}
				}
				currentNodes = descendants
			}
		} else if tok.combinator == ">" {
			var children []*Node
			seen := make(map[int]bool)
			for _, n := range currentNodes {
				for _, c := range n.ChildNodes {
					if c.NodeType == ElementNode && !seen[c.Uid] {
						seen[c.Uid] = true
						children = append(children, c)
					}
				}
			}
			currentNodes = children
		} else if tok.combinator == "+" {
			var siblings []*Node
			seen := make(map[int]bool)
			for _, n := range currentNodes {
				if n.ParentNode != nil {
					found := false
					for _, sib := range n.ParentNode.ChildNodes {
						if found {
							if sib.NodeType == ElementNode {
								if !seen[sib.Uid] {
									seen[sib.Uid] = true
									siblings = append(siblings, sib)
								}
								break
							}
							continue
						}
						if sib == n {
							found = true
						}
					}
				}
			}
			currentNodes = siblings
		} else if tok.combinator == "~" {
			var siblings []*Node
			seen := make(map[int]bool)
			for _, n := range currentNodes {
				if n.ParentNode != nil {
					found := false
					for _, sib := range n.ParentNode.ChildNodes {
						if found && sib.NodeType == ElementNode && !seen[sib.Uid] {
							seen[sib.Uid] = true
							siblings = append(siblings, sib)
						}
						if sib == n {
							found = true
						}
					}
				}
			}
			currentNodes = siblings
		}

		// Filter currentNodes by token matcher
		for _, n := range currentNodes {
			if matchesToken(n, tok) {
				nextNodes = append(nextNodes, n)
			}
		}
		currentNodes = nextNodes
		if len(currentNodes) == 0 {
			break
		}
	}

	return currentNodes
}
