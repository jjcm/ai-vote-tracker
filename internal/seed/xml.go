package seed

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"strings"
)

// The offline corpus is written in the same shape Congress.gov publishes bill
// text in — a <bill> with a <form> masthead and a <legis-body> of numbered
// <section> elements — so that the code paths exercised without an API key are
// the ones that run with it: the same XML reading, the same structural
// splitting when a bill overruns a model's context window.

// section is one section of a bill. Either Text or Subs is set.
type section struct {
	Header string
	Text   string
	Subs   []subsection
}

// subsection is a lettered subsection, optionally broken into numbered
// paragraphs.
type subsection struct {
	Header string
	Text   string
	Paras  []string
}

// bill renders sections as bill XML. chamber picks the masthead.
func billXML(number, title, chamber, longTitle string, sections []section) string {
	var b strings.Builder
	stage, house := "Introduced-in-Senate", "IN THE SENATE OF THE UNITED STATES"
	if chamber == "House" {
		stage, house = "Introduced-in-House", "IN THE HOUSE OF REPRESENTATIVES"
	}

	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	b.WriteString(`<!DOCTYPE bill PUBLIC "-//US Congress//DTDs/bill.dtd//EN" "bill.dtd">` + "\n")
	fmt.Fprintf(&b, `<bill bill-stage=%q bill-type="olc" public-private="public">`+"\n", stage)
	b.WriteString(`<metadata xmlns:dc="http://purl.org/dc/elements/1.1/">` + "\n<dublinCore>\n")
	fmt.Fprintf(&b, "<dc:title>119 %s IS: %s</dc:title>\n", number, escape(title))
	b.WriteString("<dc:type>Senate Bill</dc:type>\n</dublinCore>\n</metadata>\n")

	b.WriteString("<form>\n<distribution-code display=\"yes\">I</distribution-code>\n")
	b.WriteString("<congress>119th CONGRESS</congress>\n<session>1st Session</session>\n")
	fmt.Fprintf(&b, "<legis-num>%s</legis-num>\n", escape(number))
	fmt.Fprintf(&b, "<current-chamber>%s</current-chamber>\n", house)
	fmt.Fprintf(&b, "<official-title>%s</official-title>\n</form>\n", escape(longTitle))

	b.WriteString("<legis-body>\n")
	for i, s := range sections {
		writeSection(&b, number, i+1, s)
	}
	b.WriteString("</legis-body>\n</bill>\n")
	return b.String()
}

func writeSection(b *strings.Builder, number string, n int, s section) {
	kind := "subsequent-section"
	if n == 1 {
		kind = "section-one"
	}
	fmt.Fprintf(b, "<section id=%q section-type=%q>\n", elementID(number, fmt.Sprintf("sec%d", n)), kind)
	fmt.Fprintf(b, "<enum>%d.</enum><header>%s</header>\n", n, escape(s.Header))
	if s.Text != "" {
		fmt.Fprintf(b, "<text>%s</text>\n", escape(s.Text))
	}
	for i, sub := range s.Subs {
		fmt.Fprintf(b, "<subsection id=%q>\n", elementID(number, fmt.Sprintf("sec%dsub%d", n, i+1)))
		fmt.Fprintf(b, "<enum>(%c)</enum>", 'a'+rune(i))
		if sub.Header != "" {
			fmt.Fprintf(b, "<header>%s</header>", escape(sub.Header))
		}
		b.WriteString("\n")
		if sub.Text != "" {
			fmt.Fprintf(b, "<text>%s</text>\n", escape(sub.Text))
		}
		for j, para := range sub.Paras {
			fmt.Fprintf(b, "<paragraph id=%q><enum>(%d)</enum><text>%s</text></paragraph>\n",
				elementID(number, fmt.Sprintf("sec%dsub%dpar%d", n, i+1, j+1)), j+1, escape(para))
		}
		b.WriteString("</subsection>\n")
	}
	b.WriteString("</section>\n")
}

// elementID mimics the stable hex ids Congress.gov puts on every element.
func elementID(number, path string) string {
	sum := sha1.Sum([]byte(number + "/" + path))
	return "H" + strings.ToUpper(hex.EncodeToString(sum[:])[:31])
}

func escape(s string) string {
	return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;").Replace(strings.TrimSpace(s))
}

// definitions builds the definitions section every substantial bill carries,
// from term/meaning pairs.
func definitions(intro string, terms [][2]string) section {
	s := section{Header: "Definitions", Text: intro}
	for _, t := range terms {
		s.Subs = append(s.Subs, subsection{
			Header: t[0],
			Text:   fmt.Sprintf("The term %q means %s", t[0], t[1]),
		})
	}
	return s
}

// division is a division of an appropriations bill, which groups titles rather
// than sections. It exercises the coarser structural boundaries the splitter
// falls back to when a bill's sections are too large to take one at a time.
type division struct {
	Enum   string
	Header string
	Titles []billTitle
}

type billTitle struct {
	Enum     string
	Header   string
	Accounts []account
}

// account is one appropriation: the unit an appropriations act is written in.
type account struct {
	Agency  string
	Header  string
	Amount  string
	Purpose string
	Proviso []string
}

// appropriationsXML renders a division/title/section structure.
func appropriationsXML(number, title, chamber, longTitle string, divisions []division) string {
	var b strings.Builder
	fmt.Fprintf(&b, `<?xml version="1.0" encoding="UTF-8"?>`+"\n")
	b.WriteString(`<!DOCTYPE bill PUBLIC "-//US Congress//DTDs/bill.dtd//EN" "bill.dtd">` + "\n")
	b.WriteString(`<bill bill-stage="Introduced-in-House" bill-type="approp" public-private="public">` + "\n")
	b.WriteString(`<metadata xmlns:dc="http://purl.org/dc/elements/1.1/">` + "\n<dublinCore>\n")
	fmt.Fprintf(&b, "<dc:title>119 %s IH: %s</dc:title>\n", number, escape(title))
	b.WriteString("</dublinCore>\n</metadata>\n<form>\n")
	b.WriteString("<congress>119th CONGRESS</congress>\n<session>1st Session</session>\n")
	fmt.Fprintf(&b, "<legis-num>%s</legis-num>\n", escape(number))
	fmt.Fprintf(&b, "<current-chamber>IN THE HOUSE OF REPRESENTATIVES</current-chamber>\n")
	fmt.Fprintf(&b, "<official-title>%s</official-title>\n</form>\n", escape(longTitle))
	b.WriteString("<legis-body>\n")
	b.WriteString(`<section id="` + elementID(number, "sec1") + `" section-type="section-one">` + "\n")
	b.WriteString("<enum>1.</enum><header>Short title</header>\n<text>")
	b.WriteString(escape(`This Act may be cited as the "` + title + `".`))
	b.WriteString("</text>\n</section>\n")

	n := 100
	for _, d := range divisions {
		fmt.Fprintf(&b, "<division id=%q division-type=\"division\">\n", elementID(number, "div"+d.Enum))
		fmt.Fprintf(&b, "<enum>%s</enum><header>%s</header>\n", escape(d.Enum), escape(d.Header))
		for _, t := range d.Titles {
			fmt.Fprintf(&b, "<title id=%q>\n", elementID(number, "div"+d.Enum+"title"+t.Enum))
			fmt.Fprintf(&b, "<enum>%s</enum><header>%s</header>\n", escape(t.Enum), escape(t.Header))
			for _, a := range t.Accounts {
				n++
				fmt.Fprintf(&b, "<section id=%q section-type=\"subsequent-section\">\n", elementID(number, fmt.Sprintf("sec%d", n)))
				fmt.Fprintf(&b, "<enum>%d.</enum><header>%s—%s</header>\n", n, escape(a.Agency), escape(a.Header))
				text := fmt.Sprintf("For necessary expenses of %s for %s, %s, to remain available until September 30, 2028.",
					a.Agency, a.Purpose, a.Amount)
				for _, p := range a.Proviso {
					text += " Provided, That " + p
				}
				fmt.Fprintf(&b, "<text>%s</text>\n</section>\n", escape(text))
			}
			b.WriteString("</title>\n")
		}
		b.WriteString("</division>\n")
	}
	b.WriteString("</legis-body>\n</bill>\n")
	return b.String()
}
