package contenders

import "testing"

// Every entry has to carry the identifiers the two chambers publish, or a
// member's votes will silently never be matched to them.
func TestRosterEntriesAreComplete(t *testing.T) {
	seenBioguide := map[string]string{}
	seenLIS := map[string]string{}
	for _, c := range Roster {
		if c.Bioguide == "" || c.Name == "" || c.Surname == "" || c.Party == "" || c.State == "" {
			t.Errorf("%+v is missing a field", c)
		}
		if c.Chamber != ChamberHouse && c.Chamber != ChamberSenate {
			t.Errorf("%s sits in %q, which is not a chamber", c.Name, c.Chamber)
		}
		// Senate roll calls carry no Bioguide ID, only an LIS one, so a
		// senator without it can never be resolved from an identifier.
		if c.Chamber == ChamberSenate && c.LISID == "" {
			t.Errorf("%s has no LIS member ID", c.Name)
		}
		if c.Chamber == ChamberHouse && c.LISID != "" {
			t.Errorf("%s is a representative but carries an LIS member ID", c.Name)
		}
		if prior, dup := seenBioguide[c.Bioguide]; dup {
			t.Errorf("%s and %s share the Bioguide ID %s", prior, c.Name, c.Bioguide)
		}
		seenBioguide[c.Bioguide] = c.Name
		if c.LISID != "" {
			if prior, dup := seenLIS[c.LISID]; dup {
				t.Errorf("%s and %s share the LIS ID %s", prior, c.Name, c.LISID)
			}
			seenLIS[c.LISID] = c.Name
		}
	}
}

// The page says why the governors and cabinet officers carry no score, so
// nobody may appear on both lists.
func TestOutsidersAreNotOnTheRoster(t *testing.T) {
	if len(NotInCongress) == 0 {
		t.Fatal("the non-Congress potentials should be named")
	}
	for _, o := range NotInCongress {
		if o.Name == "" || o.Office == "" {
			t.Errorf("%+v is missing a field", o)
		}
		for _, c := range Roster {
			if c.Name == o.Name {
				t.Errorf("%s cannot be both a sitting member and an outsider", o.Name)
			}
		}
	}
}

func TestResolveByIdentifier(t *testing.T) {
	// The House Clerk stamps each legislator with a Bioguide ID.
	if c, ok := Resolve(ChamberHouse, "O000172", "Ocasio-Cortez", "NY"); !ok || c.Name != "Alexandria Ocasio-Cortez" {
		t.Errorf("resolving a Bioguide ID gave %+v %v", c, ok)
	}
	// The Senate publishes an LIS member ID instead.
	if c, ok := Resolve(ChamberSenate, "S370", "Booker", "NJ"); !ok || c.Name != "Cory Booker" {
		t.Errorf("resolving an LIS ID gave %+v %v", c, ok)
	}
	// An identifier from the wrong chamber is not a match.
	if c, ok := Resolve(ChamberHouse, "S370", "", ""); ok {
		t.Errorf("an LIS ID should not resolve against the House, got %s", c.Name)
	}
}

// Two sitting senators are named Scott, so the fallback needs the state to
// tell Rick Scott of Florida from Tim Scott of South Carolina.
func TestResolveFallsBackToSurnameAndState(t *testing.T) {
	rick, ok := Resolve(ChamberSenate, "", "Scott", "FL")
	if !ok || rick.Name != "Rick Scott" {
		t.Errorf("Scott of Florida resolved to %+v %v", rick, ok)
	}
	tim, ok := Resolve(ChamberSenate, "", "Scott", "SC")
	if !ok || tim.Name != "Tim Scott" {
		t.Errorf("Scott of South Carolina resolved to %+v %v", tim, ok)
	}
	if c, ok := Resolve(ChamberSenate, "", "Scott", ""); ok {
		t.Errorf("a surname with no state is ambiguous, got %s", c.Name)
	}
	if c, ok := Resolve(ChamberSenate, "", "Scott", "TX"); ok {
		t.Errorf("no senator named Scott sits for Texas, got %s", c.Name)
	}
}

func TestResolveRejectsMembersOffTheList(t *testing.T) {
	if c, ok := Resolve(ChamberHouse, "A000370", "Adams", "NC"); ok {
		t.Errorf("a member who is not a 2028 potential must not resolve, got %s", c.Name)
	}
	if _, ok := ByBioguide("Z999999"); ok {
		t.Error("an unknown Bioguide ID must not resolve")
	}
}

func TestTitleMatchesTheChamber(t *testing.T) {
	if got := (Candidate{Chamber: ChamberSenate}).Title(); got != "Sen." {
		t.Errorf("Senate title = %q", got)
	}
	if got := (Candidate{Chamber: ChamberHouse}).Title(); got != "Rep." {
		t.Errorf("House title = %q", got)
	}
}
