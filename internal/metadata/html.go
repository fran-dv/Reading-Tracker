package metadata

import (
	"io"
	"net/url"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// skipped holds elements whose text is not part of an article's body.
var skipped = map[atom.Atom]bool{
	atom.Script: true, atom.Style: true, atom.Noscript: true, atom.Template: true,
	atom.Nav: true, atom.Header: true, atom.Footer: true, atom.Aside: true,
}

// extractArticle reads an HTML page as an article. It is a heuristic, not a
// readability engine: the body text is the first <article>, else the first
// <main>, else <body>, minus navigation and scripts. The paste-text field
// covers the pages where this guesses wrong.
func extractArticle(r io.Reader, base *url.URL) (Result, error) {
	doc, err := html.Parse(r)
	if err != nil {
		return Result{}, err
	}

	var title, ogTitle, author, articleAuthor, image string
	var article, main, body *html.Node
	for n := range doc.Descendants() {
		if n.Type != html.ElementNode {
			continue
		}
		switch n.DataAtom {
		case atom.Title:
			if title == "" {
				title = text(n)
			}
		case atom.Meta:
			content := strings.TrimSpace(attr(n, "content"))
			switch {
			case attr(n, "property") == "og:title":
				ogTitle = content
			case attr(n, "property") == "og:image":
				image = content
			case attr(n, "name") == "author":
				author = content
			case attr(n, "property") == "article:author" && !strings.HasPrefix(content, "http"):
				articleAuthor = content // often a profile URL, which is no name
			}
		case atom.Article:
			if article == nil {
				article = n
			}
		case atom.Main:
			if main == nil {
				main = n
			}
		case atom.Body:
			body = n
		}
	}

	res := Result{Format: "article", Title: firstNonEmpty(ogTitle, title), Author: firstNonEmpty(author, articleAuthor)}
	if img, err := url.Parse(image); image != "" && err == nil {
		res.CoverURL = base.ResolveReference(img).String()
	}
	for _, root := range []*html.Node{article, main, body} {
		if root != nil {
			res.WordCount = CountWords(text(root))
			break
		}
	}
	return res, nil
}

// text joins the text under n, skipping non-content elements. Text nodes are
// separated by spaces so "<p>one</p><p>two</p>" reads as two words.
func text(n *html.Node) string {
	var b strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && skipped[n.DataAtom] {
			return
		}
		if n.Type == html.TextNode {
			b.WriteString(n.Data)
			b.WriteByte(' ')
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return strings.Join(strings.Fields(b.String()), " ")
}

func attr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
