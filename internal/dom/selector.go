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
	var tokens []selectorToken

	sel = strings.TrimSpace(sel)
	if sel == "" {
		return tokens
	}

	var parts []string
	var current strings.Builder
	inAttr := false
	inString := false
	var stringChar rune
	inParens := 0

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

		if ch == '(' {
			inParens++
			current.WriteRune(ch)
			continue
		} else if ch == ')' {
			inParens--
			if inParens < 0 {
				inParens = 0
			}
			current.WriteRune(ch)
			continue
		}

		if inParens > 0 {
			current.WriteRune(ch)
			continue
		}

		if ch == ' ' || ch == '>' || ch == '+' || ch == '~' {
			if current.Len() > 0 {
				parts = append(parts, current.String())
				current.Reset()
			}
			if ch != ' ' || (len(parts) > 0 && parts[len(parts)-1] != " " && parts[len(parts)-1] != ">" && parts[len(parts)-1] != "+" && parts[len(parts)-1] != "~") {
				parts = append(parts, string(ch))
			}
		} else {
			current.WriteRune(ch)
		}
	}
	if current.Len() > 0 {
		parts = append(parts, current.String())
	}

	var cleanedParts []string
	for i := 0; i < len(parts); i++ {
		p := strings.TrimSpace(parts[i])
		if p == "" {
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

	var current strings.Builder
	mode := 't'

	inString := false
	var stringChar rune
	inParens := 0

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

		if ch == '(' {
			inParens++
			current.WriteRune(ch)
			continue
		} else if ch == ')' {
			inParens--
			if inParens < 0 {
				inParens = 0
			}
			current.WriteRune(ch)
			if inParens == 0 && mode == ':' {
				finishCurrent()
				mode = ' '
			}
			continue
		}

		if inParens > 0 {
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

		current.WriteRune(ch)
	}
	finishCurrent()

	return tok
}


type Matcher func(*Node) bool

func compileSimpleSelector(tok selectorToken) Matcher {
	return func(node *Node) bool {
		if node.NodeType != ElementNode {
			return false
		}
		if tok.tag != "" && tok.tag != "*" && node.NodeName != tok.tag {
			return false
		}
		if tok.id != "" && node.Id != tok.id {
			return false
		}
		for _, cls := range tok.classes {
			if !node.HasClass(cls) {
				return false
			}
		}
		for _, attr := range tok.attrs {
			val, exists := node.Attributes[attr.name]
			if !exists {
				return false
			}
			if attr.op == "=" && val != attr.value {
				return false
			} else if attr.op == "^=" && !strings.HasPrefix(val, attr.value) {
				return false
			} else if attr.op == "$=" && !strings.HasSuffix(val, attr.value) {
				return false
			} else if attr.op == "*=" && !strings.Contains(val, attr.value) {
				return false
			}
		}
		for _, pseudo := range tok.pseudos {
			if pseudo == "first-child" {
				if node.PreviousElementSibling() != nil {
					return false
				}
			} else if pseudo == "last-child" {
				if node.NextElementSibling() != nil {
					return false
				}
			} else if strings.HasPrefix(pseudo, "not(") && strings.HasSuffix(pseudo, ")") {
				innerSel := pseudo[4 : len(pseudo)-1]
				innerMatcher := compileSelector(innerSel)
				if innerMatcher(node) {
					return false
				}
			}
			// basic nth-child etc could be added here
		}
		return true
	}
}

func compileSelector(selector string) Matcher {
	tokens := parseSelector(selector)
	if len(tokens) == 0 {
		return func(*Node) bool { return false }
	}

	var m Matcher = compileSimpleSelector(tokens[0])

	for i := 1; i < len(tokens); i++ {
		leftMatcher := m
		rightMatcher := compileSimpleSelector(tokens[i])
		comb := tokens[i].combinator

		if comb == ">" {
			m = func(n *Node) bool {
				if !rightMatcher(n) { return false }
				p := n.ParentNode
				if p == nil || p.NodeType != ElementNode { return false }
				return leftMatcher(p)
			}
		} else if comb == " " || comb == "" {
			m = func(n *Node) bool {
				if !rightMatcher(n) { return false }
				p := n.ParentNode
				for p != nil && p.NodeType == ElementNode {
					if leftMatcher(p) { return true }
					p = p.ParentNode
				}
				return false
			}
		} else if comb == "+" {
			m = func(n *Node) bool {
				if !rightMatcher(n) { return false }
				p := n.PreviousElementSibling()
				if p == nil || p.NodeType != ElementNode { return false }
				return leftMatcher(p)
			}
		} else if comb == "~" {
			m = func(n *Node) bool {
				if !rightMatcher(n) { return false }
				p := n.PreviousElementSibling()
				for p != nil && p.NodeType == ElementNode {
					if leftMatcher(p) { return true }
					p = p.PreviousElementSibling()
				}
				return false
			}
		}
	}
	return m
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

	matcher := compileSelector(selector)
	var results []*Node

	var dfs func(n *Node)
	dfs = func(n *Node) {
		if n.NodeType == ElementNode {
			if matcher(n) {
				results = append(results, n)
			}
		}
		for _, c := range n.ChildNodes {
			dfs(c)
		}
	}

	// Start searching from descendants of root
	for _, c := range root.ChildNodes {
		dfs(c)
	}

	return results
}
