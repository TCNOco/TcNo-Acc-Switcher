package confirmationapi

import (
	"encoding/json"
	"strings"
	"unicode"

	nethtml "golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

const (
	maxDetailsFields = 64
	maxDetailsText   = 2048
)

func decodeDetails(raw []byte) ([]TextField, *TradeDetails, *ListingDetails, error) {
	var envelope struct {
		Success  bool   `json:"success"`
		NeedAuth bool   `json:"needauth"`
		Message  string `json:"message"`
		HTML     string `json:"html"`
	}
	if err := decodeEnvelope(raw, &envelope); err != nil {
		return nil, nil, nil, &Error{Kind: FailureFailed}
	}
	if envelope.NeedAuth {
		return nil, nil, nil, &Error{Kind: FailureReauth}
	}
	if !envelope.Success || len(envelope.HTML) == 0 || len(envelope.HTML) > maxDetailsResponseBytes {
		return nil, nil, nil, &Error{Kind: FailureFailed}
	}
	// A listing that reads cleanly replaces the text fields rather than joining
	// them: everything the generic parser would return for one is either in the
	// listing already or is the boilerplate this exists to drop.
	if listing := decodeListing(envelope.HTML); listing != nil {
		return nil, nil, listing, nil
	}
	contextNode := &nethtml.Node{Type: nethtml.ElementNode, DataAtom: atom.Body, Data: "body"}
	nodes, err := nethtml.ParseFragment(strings.NewReader(envelope.HTML), contextNode)
	if err != nil {
		return nil, nil, nil, &Error{Kind: FailureFailed}
	}
	fields := make([]TextField, 0, 16)
	seen := make(map[string]struct{})
	invalid := false
	for _, node := range nodes {
		collectDetailBlocks(node, &fields, seen, &invalid)
		if invalid || len(fields) > maxDetailsFields {
			return nil, nil, nil, &Error{Kind: FailureFailed}
		}
	}
	if len(fields) == 0 {
		text := visibleNodeText(nodes)
		if len(text) > maxDetailsText {
			return nil, nil, nil, &Error{Kind: FailureFailed}
		}
		if text != "" {
			fields = append(fields, TextField{Label: "Details", Value: text})
		}
	}
	return fields, decodeTrade(envelope.HTML), nil, nil
}

func collectDetailBlocks(node *nethtml.Node, fields *[]TextField, seen map[string]struct{}, invalid *bool) {
	if node == nil || *invalid || len(*fields) > maxDetailsFields || excludedNode(node) {
		return
	}
	if node.Type == nethtml.ElementNode {
		switch node.DataAtom {
		case atom.Tr:
			var cells []string
			for child := node.FirstChild; child != nil; child = child.NextSibling {
				if child.DataAtom == atom.Td || child.DataAtom == atom.Th {
					if text := visibleText(child); text != "" {
						cells = append(cells, text)
					}
				}
			}
			if len(cells) >= 2 {
				addField(fields, seen, cells[0], strings.Join(cells[1:], " "), invalid)
				return
			}
		case atom.Li, atom.P, atom.H1, atom.H2, atom.H3, atom.H4:
			if text := visibleText(node); text != "" {
				label, value := splitDetail(text)
				addField(fields, seen, label, value, invalid)
				return
			}
		}
	}
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		collectDetailBlocks(child, fields, seen, invalid)
	}
}

func splitDetail(text string) (string, string) {
	if before, after, found := strings.Cut(text, ":"); found && len(before) <= 128 && strings.TrimSpace(after) != "" {
		return strings.TrimSpace(before), strings.TrimSpace(after)
	}
	return "Detail", text
}

func addField(fields *[]TextField, seen map[string]struct{}, label, value string, invalid *bool) {
	label = collapseText(label)
	value = collapseText(value)
	if len(label) > maxTextOutput || len(value) > maxDetailsText {
		*invalid = true
		return
	}
	if label == "" || value == "" {
		return
	}
	keyBytes, _ := json.Marshal([]string{label, value})
	key := string(keyBytes)
	if _, exists := seen[key]; exists {
		return
	}
	seen[key] = struct{}{}
	*fields = append(*fields, TextField{Label: label, Value: value})
}

func visibleNodeText(nodes []*nethtml.Node) string {
	var builder strings.Builder
	for _, node := range nodes {
		appendVisibleText(&builder, node)
	}
	return collapseText(builder.String())
}

func visibleText(node *nethtml.Node) string {
	var builder strings.Builder
	appendVisibleText(&builder, node)
	return collapseText(builder.String())
}

func appendVisibleText(builder *strings.Builder, node *nethtml.Node) {
	if node == nil || excludedNode(node) {
		return
	}
	if node.Type == nethtml.TextNode {
		builder.WriteString(node.Data)
		builder.WriteByte(' ')
	}
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		appendVisibleText(builder, child)
	}
}

func excludedNode(node *nethtml.Node) bool {
	return node.Type == nethtml.CommentNode || (node.Type == nethtml.ElementNode &&
		(node.DataAtom == atom.Script || node.DataAtom == atom.Style || node.DataAtom == atom.Noscript || node.DataAtom == atom.Svg))
}

func collapseText(value string) string {
	return strings.Join(strings.FieldsFunc(value, unicode.IsSpace), " ")
}
