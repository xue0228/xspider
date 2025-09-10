package extractor

import (
	"strings"

	"github.com/antchfx/htmlquery"
	"golang.org/x/net/html"
)

func ExtractSingleNode(node *html.Node, expr string, attr string, firstOnly bool) []string {
	if node == nil {
		return nil
	}

	var nodes []*html.Node
	if expr == "" {
		nodes = append(nodes, node)
	} else {
		nodesFind, err := htmlquery.QueryAll(node, expr)
		if err != nil {
			return nil
		}
		nodes = append(nodes, nodesFind...)
	}
	var res []string
	for _, nd := range nodes {
		var text string
		if attr == "" {
			text = htmlquery.InnerText(nd)
		} else {
			text = htmlquery.SelectAttr(nd, attr)
		}
		res = append(res, strings.TrimSpace(text))
		if firstOnly {
			return res
		}
	}
	return res
}
