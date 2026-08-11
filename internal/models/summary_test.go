package models

import "testing"

func TestSummaryWithoutTitleEcho(t *testing.T) {
	const title = "Border Security and Asylum Reform Act of 2025"
	const body = "This bill strengthens border security measures and reforms the asylum process."

	tests := []struct {
		name    string
		title   string
		summary string
		want    string
	}{{
		// What Congress.gov's summary feed actually sends: the short title as
		// its own paragraph ahead of the summary proper.
		name:    "title paragraph",
		title:   title,
		summary: title + "\n\n" + body,
		want:    body,
	}, {
		// The same summary after its paragraphs have been flattened into one
		// line, which is how it reaches the bill cards.
		name:    "flattened title paragraph",
		title:   title,
		summary: title + " " + body,
		want:    body,
	}, {
		name:    "title as its own sentence",
		title:   title,
		summary: title + ". " + body,
		want:    body,
	}, {
		name:    "title set off by a dash",
		title:   title,
		summary: title + " \u2014 " + body,
		want:    body,
	}, {
		// Congress is inconsistent about the year, so the echo and the title
		// need not carry it identically.
		name:    "echo without the title's year",
		title:   title,
		summary: "Border Security and Asylum Reform Act. " + body,
		want:    body,
	}, {
		name:    "echo carrying a year the title omits",
		title:   "Consolidated Appropriations Act",
		summary: "Consolidated Appropriations Act, 2026\n\n" + body,
		want:    body,
	}, {
		name:    "case and punctuation differences",
		title:   title,
		summary: "BORDER SECURITY AND ASYLUM-REFORM ACT OF 2025\n" + body,
		want:    body,
	}, {
		name:    "echo introduced as a citation",
		title:   title,
		summary: "This bill may be cited as the " + title + ". " + body,
		want:    body,
	}, {
		// A real summary is left exactly as it is.
		name:    "no echo",
		title:   "American Energy Independence Act",
		summary: "Expands domestic energy production and accelerates permitting reforms.",
		want:    "Expands domestic energy production and accelerates permitting reforms.",
	}, {
		// "This bill …" is the body of most CRS summaries; it is only dropped
		// when it is restating the title, which here it is not.
		name:    "opening this-bill sentence that is not an echo",
		title:   title,
		summary: body,
		want:    body,
	}, {
		// The title is the sentence's subject, so cutting it would leave a verb
		// with nothing to attach to.
		name:    "title running on into the sentence",
		title:   title,
		summary: title + " would create a new office within the Department.",
		want:    title + " would create a new office within the Department.",
	}, {
		// Nothing better to show than the repetition.
		name:    "summary is only the echo",
		title:   title,
		summary: title,
		want:    title,
	}, {
		name:    "echo leaving too little behind",
		title:   title,
		summary: title + ". Reforms asylum.",
		want:    title + ". Reforms asylum.",
	}, {
		// Short titles match by accident too easily to strip.
		name:    "two word title",
		title:   "Farm Act",
		summary: "Farm Act. " + body,
		want:    "Farm Act. " + body,
	}, {
		name:    "a word the title happens to start with",
		title:   "Protecting Children Online Safety Act",
		summary: "Protecting children is the stated aim, and the bill sets platform duties to that end.",
		want:    "Protecting children is the stated aim, and the bill sets platform duties to that end.",
	}, {
		name:    "empty summary",
		title:   title,
		summary: "",
		want:    "",
	}, {
		name:    "empty title",
		title:   "",
		summary: body,
		want:    body,
	}}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := SummaryWithoutTitleEcho(tc.title, tc.summary); got != tc.want {
				t.Errorf("SummaryWithoutTitleEcho(%q, %q)\n got %q\nwant %q", tc.title, tc.summary, got, tc.want)
			}
		})
	}
}
