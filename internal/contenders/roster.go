// Package contenders holds the curated list of sitting members of Congress who
// are frequently named in the national press as potential 2028 presidential
// candidates, and the well-known potentials who hold no seat.
//
// # Where this list comes from
//
// This is an editorial list, not an official one. There is no 2028 candidate
// registry to read from: the Federal Election Commission only knows about
// people who have filed, and nobody on this list has. The membership standard
// is "frequently mentioned as a 2028 presidential potential in national press
// as of 2026" — the recurring names in cycle handicapping, early-state polling
// and shortlist coverage. Reasonable people would add and remove names, so the
// list lives in code where a diff can be argued with.
//
// # Why only Congress
//
// The page compares an AI model's Yes/No verdicts against real recorded floor
// votes, so a potential candidate can only appear if they can cast one.
// Governors, the Vice President and cabinet officers are named in the same
// coverage and are just as plausible as candidates, but they have no roll call
// record to compare against. They are listed in [NotInCongress] so the page can
// say why they are absent rather than inventing a score for them.
//
// # Identifiers
//
// Every member is keyed by their Bioguide ID, which is what the House Clerk
// publishes on each recorded vote. The Senate's roll call XML does not carry
// Bioguide IDs — it identifies senators by an LIS member ID — so senators also
// carry that, and both chambers fall back to matching on surname and state.
package contenders

import "strings"

// Chambers a contender can sit in.
const (
	ChamberHouse  = "House"
	ChamberSenate = "Senate"
)

// Candidate is one sitting member of Congress on the 2028 watch list.
type Candidate struct {
	// Bioguide is the member's Biographical Directory ID, the identifier the
	// House Clerk prints on every recorded vote as name-id.
	Bioguide string `json:"bioguide"`
	// LISID is the Senate's own member identifier, which is what senate.gov
	// roll call XML carries instead of a Bioguide ID. Empty for House members.
	LISID string `json:"lisId,omitempty"`
	Name  string `json:"name"`
	// Surname is the member's name as the roll call feeds print it, used to
	// match a vote when neither identifier is present.
	Surname string `json:"-"`
	Party   string `json:"party"`
	State   string `json:"state"`
	Chamber string `json:"chamber"`
	// Note is the one line the page shows about why they are on the list.
	Note string `json:"note,omitempty"`
}

// Title is the honorific the page puts in front of the name.
func (c Candidate) Title() string {
	if c.Chamber == ChamberSenate {
		return "Sen."
	}
	return "Rep."
}

// Roster is the watch list: sitting senators and representatives named in
// national 2028 coverage during 2026. Senate LIS IDs are as published in the
// senate.gov roll call XML; House Bioguide IDs are as published by the Clerk.
var Roster = []Candidate{
	// Senate Democrats.
	{Bioguide: "B001288", LISID: "S370", Name: "Cory Booker", Surname: "Booker", Party: "D", State: "NJ", Chamber: ChamberSenate, Note: "Ran in 2020 and stayed in the 2028 conversation throughout 2026."},
	{Bioguide: "G000574", LISID: "S432", Name: "Ruben Gallego", Surname: "Gallego", Party: "D", State: "AZ", Chamber: ChamberSenate, Note: "Named repeatedly as the party's working-class-outreach option."},
	{Bioguide: "K000377", LISID: "S406", Name: "Mark Kelly", Surname: "Kelly", Party: "D", State: "AZ", Chamber: ChamberSenate, Note: "A perennial swing-state shortlist name."},
	{Bioguide: "O000174", LISID: "S414", Name: "Jon Ossoff", Surname: "Ossoff", Party: "D", State: "GA", Chamber: ChamberSenate, Note: "Floated after his Georgia re-election campaign drew national money."},
	{Bioguide: "S001208", LISID: "S436", Name: "Elissa Slotkin", Surname: "Slotkin", Party: "D", State: "MI", Chamber: ChamberSenate, Note: "Her post-2024 \"war plan\" speeches put her on most 2028 lists."},
	{Bioguide: "W000790", LISID: "S415", Name: "Raphael Warnock", Surname: "Warnock", Party: "D", State: "GA", Chamber: ChamberSenate, Note: "Mentioned as a southern general-election option."},
	{Bioguide: "M001169", LISID: "S364", Name: "Chris Murphy", Surname: "Murphy", Party: "D", State: "CT", Chamber: ChamberSenate, Note: "One of the loudest anti-authoritarian voices of the term."},
	{Bioguide: "V000128", LISID: "S390", Name: "Chris Van Hollen", Surname: "Van Hollen", Party: "D", State: "MD", Chamber: ChamberSenate, Note: "Raised after his foreign-policy confrontations with the administration."},

	// Senate Republicans.
	{Bioguide: "C001098", LISID: "S355", Name: "Ted Cruz", Surname: "Cruz", Party: "R", State: "TX", Chamber: ChamberSenate, Note: "Ran in 2016 and has never left the shortlists."},
	{Bioguide: "H001089", LISID: "S399", Name: "Josh Hawley", Surname: "Hawley", Party: "R", State: "MO", Chamber: ChamberSenate, Note: "The populist-right lane's most cited senator."},
	{Bioguide: "S001217", LISID: "S404", Name: "Rick Scott", Surname: "Scott", Party: "R", State: "FL", Chamber: ChamberSenate, Note: "Self-funding Florida option named in early handicapping."},
	{Bioguide: "P000603", LISID: "S348", Name: "Rand Paul", Surname: "Paul", Party: "R", State: "KY", Chamber: ChamberSenate, Note: "Still the libertarian lane's default name."},
	{Bioguide: "C001095", LISID: "S374", Name: "Tom Cotton", Surname: "Cotton", Party: "R", State: "AR", Chamber: ChamberSenate, Note: "A fixture of national-security-hawk shortlists."},
	{Bioguide: "S001184", LISID: "S365", Name: "Tim Scott", Surname: "Scott", Party: "R", State: "SC", Chamber: ChamberSenate, Note: "Ran in 2024 and is still named for 2028."},

	// House. Few sitting representatives are named often enough to qualify:
	// the modern path to a nomination runs through statewide office, so this
	// side of the list is short on purpose.
	{Bioguide: "O000172", Name: "Alexandria Ocasio-Cortez", Surname: "Ocasio-Cortez", Party: "D", State: "NY", Chamber: ChamberHouse, Note: "Tops most left-lane 2028 polling."},
	{Bioguide: "K000389", Name: "Ro Khanna", Surname: "Khanna", Party: "D", State: "CA", Chamber: ChamberHouse, Note: "Has campaigned in early states since 2025."},
	{Bioguide: "M001184", Name: "Thomas Massie", Surname: "Massie", Party: "R", State: "KY", Chamber: ChamberHouse, Note: "Named in libertarian-lane coverage after breaking with his party."},
}

// Outsider is a frequently named 2028 potential who cannot cast a floor vote.
type Outsider struct {
	Name   string `json:"name"`
	Office string `json:"office"`
}

// NotInCongress is the rest of the shortlist: names that come up as often as
// anyone above but hold no seat, so there is no voting record to compare a
// model against. The page names them and says so rather than scoring them.
var NotInCongress = []Outsider{
	{Name: "Gavin Newsom", Office: "Governor of California"},
	{Name: "Josh Shapiro", Office: "Governor of Pennsylvania"},
	{Name: "Gretchen Whitmer", Office: "Governor of Michigan"},
	{Name: "Wes Moore", Office: "Governor of Maryland"},
	{Name: "Andy Beshear", Office: "Governor of Kentucky"},
	{Name: "Pete Buttigieg", Office: "former Secretary of Transportation"},
	{Name: "Kamala Harris", Office: "former Vice President"},
	{Name: "JD Vance", Office: "Vice President"},
	{Name: "Marco Rubio", Office: "Secretary of State"},
	{Name: "Ron DeSantis", Office: "Governor of Florida"},
}

// ByBioguide looks up a contender.
func ByBioguide(id string) (Candidate, bool) {
	for _, c := range Roster {
		if strings.EqualFold(c.Bioguide, id) {
			return c, true
		}
	}
	return Candidate{}, false
}

// Resolve maps one line of a roll call onto a contender. The identifier is
// whichever one that chamber's feed publishes — a Bioguide ID from the House
// Clerk, an LIS member ID from the Senate — and surname with state is the
// fallback for a feed that carries neither. Two sitting senators share the
// surname Scott, so a surname on its own is never enough.
func Resolve(chamber, id, surname, state string) (Candidate, bool) {
	id = strings.TrimSpace(id)
	for _, c := range Roster {
		if c.Chamber != chamber {
			continue
		}
		if id != "" && (strings.EqualFold(id, c.Bioguide) || (c.LISID != "" && strings.EqualFold(id, c.LISID))) {
			return c, true
		}
	}
	surname, state = strings.TrimSpace(surname), strings.TrimSpace(state)
	if surname == "" || state == "" {
		return Candidate{}, false
	}
	for _, c := range Roster {
		if c.Chamber == chamber && strings.EqualFold(c.Surname, surname) && strings.EqualFold(c.State, state) {
			return c, true
		}
	}
	return Candidate{}, false
}
