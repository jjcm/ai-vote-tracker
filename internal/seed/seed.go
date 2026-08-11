// Package seed provides the offline bill corpus used when no Congress.gov API
// key is configured. The bills are realistic composites of recent House and
// Senate legislation, written as bill XML in the shape Congress.gov publishes —
// long enough, and structured enough, that the offline corpus exercises the
// same reading, splitting and digesting the live corpus does.
package seed

import (
	"time"

	"github.com/pwnies/ai-vote-tracker/internal/models"
)

func day(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

// Bills returns the seed corpus, newest first.
//
// IdeologyScore is a hand-assigned position on a -1.0 (progressive) to +1.0
// (conservative) axis. A stored score of exactly 0 means "not yet scored", so
// genuinely centrist bills carry a small non-zero value instead.
func Bills() []models.Bill {
	bills := []models.Bill{
		borderSecurity(),
		energyIndependence(),
		veteransEducation(),
		childrenOnline(),
		drugCosts(),
		aiResearch(),
		cybersecurity(),
		affordableHousing(),
		smallBusinessTax(),
		farmland(),
		elections(),
		cleanWater(),
		appropriations(),
	}

	for i := range bills {
		b := &bills[i]
		b.Source = models.SourceSeed
		b.StatusCategory = models.StatusCategory(b.Status)
		b.TextSource = models.TextSourceSeedXML
		b.TextFormat = "Formatted XML"
		if b.TextVersion == "" {
			b.TextVersion = "Introduced in " + b.Chamber
		}
	}
	return bills
}

// shortTitle is the section 1 every bill opens with.
func shortTitle(name string) section {
	return section{
		Header: "Short title",
		Text:   `This Act may be cited as the "` + name + `".`,
	}
}

func borderSecurity() models.Bill {
	const name = "Border Security and Asylum Reform Act of 2025"
	return models.Bill{
		ID:      "s-1264",
		Number:  "S. 1264",
		Title:   name,
		Chamber: models.ChamberSenate,
		Status:  "Read twice and referred to the Committee on Homeland Security",
		Summary: "This bill strengthens border security measures, expands expedited removal authority, and reforms the asylum process to improve efficiency and integrity while ensuring protections for individuals with valid claims.",
		FullText: billXML("S. 1264", name, models.ChamberSenate,
			"To secure the southern border, reform the asylum adjudication system, and for other purposes.",
			[]section{
				shortTitle(name),
				definitions("In this Act:", [][2]string{
					{"Secretary", "the Secretary of Homeland Security."},
					{"covered sector", "a Border Patrol sector in which apprehensions exceeded 25,000 in any of the 3 preceding fiscal years."},
					{"credible fear determination", "a determination under section 235(b)(1)(B) of the Immigration and Nationality Act as amended by this Act."},
					{"unaccompanied child", "an individual described in section 462(g)(2) of the Homeland Security Act of 2002."},
				}),
				{
					Header: "Border infrastructure and technology",
					Subs: []subsection{
						{
							Header: "Authorization",
							Text:   "There is authorized to be appropriated to the Secretary $8,400,000,000 for fiscal years 2026 through 2030 for physical barriers, autonomous surveillance towers, unattended ground sensors, tethered aerostats, and non-intrusive inspection systems at ports of entry.",
						},
						{
							Header: "Allocation",
							Paras: []string{
								"Not less than 15 percent of amounts appropriated under subsection (a) shall be used for contraband detection at land ports of entry, with priority for lanes with the highest volume of commercial traffic.",
								"Not less than 10 percent shall be used for surveillance and detection capability in covered sectors that lack road access.",
								"Not more than 5 percent may be obligated for program management and administration.",
							},
						},
						{
							Header: "Environmental compliance",
							Text:   "Construction under this section shall comply with applicable Federal environmental law, except that the Secretary may waive the application of a requirement upon a written finding, published in the Federal Register, that the waiver is necessary to respond to an imminent threat to life.",
						},
					},
				},
				{
					Header: "Personnel",
					Subs: []subsection{
						{
							Header: "Hiring",
							Text:   "The Secretary shall hire not fewer than 3,000 additional Border Patrol agents, 1,200 Customs and Border Protection officers, and 500 asylum officers not later than 4 years after the date of enactment of this Act.",
						},
						{
							Header: "Retention",
							Text:   "The Secretary shall establish retention bonuses of not more than 25 percent of base pay for agents assigned to remote sectors, and shall report annually on attrition by sector, grade, and years of service.",
						},
						{
							Header: "Polygraph waiver",
							Text:   "The Secretary may waive the polygraph examination requirement for an applicant who is a veteran or a current State or local law enforcement officer in good standing, provided that a full background investigation is completed.",
						},
					},
				},
				{
					Header: "Expedited removal",
					Subs: []subsection{
						{
							Header: "In general",
							Text:   "Section 235(b)(1) of the Immigration and Nationality Act (8 U.S.C. 1225(b)(1)) is amended to authorize the expedited removal of an alien encountered anywhere in the United States who cannot establish continuous physical presence in the United States for the 2-year period preceding the encounter.",
						},
						{
							Header: "Fear of persecution",
							Text:   "An alien subject to expedited removal who expresses a fear of persecution or torture shall receive a credible fear interview not later than 7 days after such expression, and may not be removed before that interview is conducted.",
						},
						{
							Header: "Exclusions",
							Text:   "Expedited removal under this section may not be applied to an unaccompanied child or to an alien who has been admitted as a lawful permanent resident.",
						},
					},
				},
				{
					Header: "Credible fear standard",
					Subs: []subsection{
						{
							Header: "Standard",
							Text:   `The credible fear standard is raised from "significant possibility" to "more likely than not" that the alien can establish eligibility for asylum under section 208 of the Immigration and Nationality Act.`,
						},
						{
							Header: "Review",
							Text:   "An alien found not to have a credible fear may request review by an immigration judge within 48 hours of the determination, and the review shall be completed within 10 days by video conference if necessary.",
						},
					},
				},
				{
					Header: "Asylum adjudication timelines",
					Subs: []subsection{
						{
							Header: "Deadline",
							Text:   "The Attorney General shall complete initial asylum adjudications not later than 180 days after the application is filed, and shall increase the immigration judge corps by 250 judges and the supporting law clerk corps proportionately to meet that deadline.",
						},
						{
							Header: "Work authorization",
							Text:   "Employment authorization for an asylum applicant may not be issued earlier than 180 days after the application is filed, except that the 180-day period shall be tolled for any delay attributable to the Government.",
						},
						{
							Header: "Frivolous filings",
							Text:   "An application found by an immigration judge to be knowingly frivolous shall render the applicant permanently ineligible for discretionary relief, after notice and an opportunity to respond.",
						},
					},
				},
				{
					Header: "Detention capacity and alternatives",
					Subs: []subsection{
						{
							Header: "Capacity",
							Text:   "The Secretary shall maintain not fewer than 50,000 detention beds, of which not fewer than 8,000 shall be suitable for family units under standards issued by the Secretary in consultation with the Secretary of Health and Human Services.",
						},
						{
							Header: "Alternatives to detention",
							Text:   "The Secretary shall operate a case management program, including legal orientation and appearance assistance, for aliens released pending adjudication, and shall report the appearance rate of participants each fiscal year.",
						},
					},
				},
				{
					Header: "Protections",
					Text:   "Nothing in this Act shall be construed to limit access to counsel at no expense to the Government, to authorize the removal of an unaccompanied child without a hearing before an immigration judge, to permit the return of an alien to a country where the alien is more likely than not to be tortured, or to abrogate obligations of the United States under the 1967 Protocol Relating to the Status of Refugees.",
				},
				{
					Header: "Reporting",
					Subs: []subsection{
						{
							Header: "Annual report",
							Text:   "The Secretary shall submit to Congress an annual report on removals, credible fear determinations, detention bed utilization, and case backlogs, disaggregated by sector and nationality.",
						},
						{
							Header: "Inspector General review",
							Text:   "The Inspector General of the Department of Homeland Security shall review compliance with the interview deadlines established by this Act and report findings to Congress every 2 years.",
						},
					},
				},
				{
					Header: "Effective date and severability",
					Subs: []subsection{
						{
							Header: "Effective date",
							Text:   "This Act and the amendments made by this Act take effect 180 days after the date of enactment, except that section 4 takes effect on the date of enactment.",
						},
						{
							Header: "Severability",
							Text:   "If any provision of this Act is held invalid, the remainder of the Act and the application of the provision to other persons or circumstances shall not be affected.",
						},
					},
				},
			}),
		PolicyArea:     "Immigration",
		Sponsor:        "Sen. Marshall, Roger [R-KS]",
		SponsorParty:   "R",
		IdeologyScore:  0.62,
		IntroducedDate: day(2025, time.April, 2),
		UpdatedAt:      day(2025, time.May, 8),
	}
}

func energyIndependence() models.Bill {
	const name = "American Energy Independence Act"
	return models.Bill{
		ID:      "hr-3721",
		Number:  "H.R. 3721",
		Title:   name,
		Chamber: models.ChamberHouse,
		Status:  "Referred to the Subcommittee on Energy and Mineral Resources",
		Summary: "Expands domestic energy production, accelerates permitting reforms, and streamlines energy infrastructure development.",
		FullText: billXML("H.R. 3721", name, models.ChamberHouse,
			"To expand domestic energy production, reform Federal permitting, and for other purposes.",
			[]section{
				shortTitle(name),
				definitions("In this Act:", [][2]string{
					{"covered energy project", "a project for the production, processing, transmission, or storage of energy, or for the extraction or processing of a critical mineral, that requires a Federal authorization."},
					{"Federal authorization", "a license, permit, approval, finding, determination, or other administrative decision required under Federal law to site, construct, or operate a covered energy project."},
					{"lead agency", "the Federal agency designated under section 4(c) to coordinate the environmental review of a covered energy project."},
				}),
				{
					Header: "Federal leasing",
					Subs: []subsection{
						{
							Header: "Onshore and offshore sales",
							Text:   "The Secretary of the Interior shall hold not fewer than four onshore oil and gas lease sales per year in each State with available Federal acreage, and shall resume quarterly offshore lease sales in the Gulf of America and Cook Inlet planning areas.",
						},
						{
							Header: "Cancellation",
							Text:   "A lease sale may not be cancelled, delayed, or reduced in acreage except upon a written finding, published not later than 30 days before the scheduled sale, of a material defect in the sale notice.",
						},
						{
							Header: "Royalty rates",
							Text:   "The royalty rate for onshore competitive leases issued under this section shall be 16.67 percent, and the Secretary shall deposit the Federal share of receipts in accordance with section 35 of the Mineral Leasing Act.",
						},
					},
				},
				{
					Header: "Permitting reform",
					Subs: []subsection{
						{
							Header: "Review deadlines",
							Text:   "Environmental reviews under the National Environmental Policy Act of 1969 for covered energy projects shall be completed within 2 years for an environmental impact statement and 1 year for an environmental assessment, with a 300-page limit on the statement exclusive of appendices.",
						},
						{
							Header: "Judicial review",
							Text:   "A claim arising under Federal law seeking judicial review of a Federal authorization for a covered energy project shall be filed not later than 150 days after publication of the notice of final agency action, and a court may not enjoin construction without a finding of irreparable harm.",
						},
						{
							Header: "Lead agency",
							Text:   "A single lead agency shall prepare one environmental document and issue one combined record of decision for each covered energy project, and each cooperating agency shall adopt that document unless it finds on the record that the document is inadequate for its own decision.",
						},
					},
				},
				{
					Header: "Transmission and pipelines",
					Subs: []subsection{
						{
							Header: "Certificate deadlines",
							Text:   "The Federal Energy Regulatory Commission shall act on an application for a certificate of public convenience and necessity for a natural gas pipeline within 12 months after the application is complete.",
						},
						{
							Header: "Backstop siting",
							Text:   "The Commission is granted backstop siting authority for interstate electric transmission lines in national interest electric transmission corridors designated by the Secretary of Energy, exercisable one year after a State authority has failed to act.",
						},
						{
							Header: "Cost allocation",
							Text:   "The Commission shall allocate the costs of a facility sited under subsection (b) to the load-serving entities that benefit from the facility in proportion to those benefits.",
						},
					},
				},
				{
					Header: "Critical minerals",
					Subs: []subsection{
						{
							Header: "Designation",
							Text:   "Domestic mining and processing projects for lithium, cobalt, nickel, copper, graphite, and rare earth elements are designated covered energy projects for purposes of section 4.",
						},
						{
							Header: "National assessment",
							Text:   "The Secretary shall complete, not later than 18 months after the date of enactment, a national assessment of critical mineral reserves, including an inventory of mine waste that may be reprocessed.",
						},
						{
							Header: "Mine permitting",
							Text:   "The Secretary shall establish a single point of contact for each covered mining project and shall publish the schedule and status of each pending authorization on a public dashboard.",
						},
					},
				},
				{
					Header: "Exports",
					Text:   "An application to export liquefied natural gas to a country that is not a party to a free trade agreement with the United States shall be deemed consistent with the public interest and granted without modification unless the Secretary of Energy makes a contrary finding, supported by substantial evidence, within 90 days after the close of the comment period.",
				},
				{
					Header: "Grid reliability",
					Subs: []subsection{
						{
							Header: "Retirement review",
							Text:   "A dispatchable generating unit of more than 200 megawatts may not be retired until the relevant reliability organization certifies that the retirement will not reduce the reserve margin below the applicable planning standard.",
						},
						{
							Header: "Interconnection",
							Text:   "Each transmission provider shall study interconnection requests in clusters and shall complete each cluster study within 300 days, subject to a refundable deposit set by the Commission.",
						},
					},
				},
				{
					Header: "Repeals",
					Text:   "The methane waste emissions charge established under section 136 of the Clean Air Act is repealed, and amounts previously collected under that section shall be refunded to the payers within 1 year.",
				},
				{
					Header: "Rules of construction",
					Text:   "Nothing in this Act shall be construed to waive the requirements of the Endangered Species Act of 1973, to limit the authority of a State under section 401 of the Federal Water Pollution Control Act, or to authorize a covered energy project on land held in trust for an Indian Tribe without the consent of that Tribe.",
				},
				{
					Header: "Reports",
					Text:   "The Comptroller General shall report to Congress not later than 3 years after the date of enactment, and every 2 years thereafter, on the effect of this Act on permitting timelines, domestic production volumes, and litigation outcomes.",
				},
			}),
		PolicyArea:     "Energy",
		Sponsor:        "Rep. Westerman, Bruce [R-AR]",
		SponsorParty:   "R",
		IdeologyScore:  0.58,
		IntroducedDate: day(2025, time.March, 18),
		UpdatedAt:      day(2025, time.May, 7),
	}
}

func veteransEducation() models.Bill {
	const name = "Veterans' Education and Opportunity Act"
	return models.Bill{
		ID:      "s-1198",
		Number:  "S. 1198",
		Title:   name,
		Chamber: models.ChamberSenate,
		Status:  "Read twice and referred to the Committee on Veterans' Affairs",
		Summary: "Improves access to education, job training, and support services for veterans and their families.",
		FullText: billXML("S. 1198", name, models.ChamberSenate,
			"To improve the educational assistance programs of the Department of Veterans Affairs, and for other purposes.",
			[]section{
				shortTitle(name),
				definitions("In this Act:", [][2]string{
					{"Secretary", "the Secretary of Veterans Affairs."},
					{"covered veteran", "a veteran entitled to educational assistance under chapter 33 of title 38, United States Code."},
					{"covered institution", "an educational institution approved for purposes of chapter 36 of title 38, United States Code."},
				}),
				{
					Header: "Restoration of entitlement",
					Subs: []subsection{
						{
							Header: "In general",
							Text:   "A covered veteran whose program of education is suspended or closed by the institution shall have the corresponding months of entitlement under chapter 33 of title 38, United States Code, restored.",
						},
						{
							Header: "Housing stipend",
							Text:   "A veteran described in subsection (a) shall continue to receive the monthly housing stipend for a period of not more than 4 months while transferring to another covered institution.",
						},
						{
							Header: "Debt relief",
							Text:   "The Secretary shall waive any debt owed by a veteran arising from an overpayment attributable to the closure of a covered institution.",
						},
					},
				},
				{
					Header: "Apprenticeship and credentialing",
					Subs: []subsection{
						{
							Header: "Eligible programs",
							Text:   "Entitlement may be used for registered apprenticeships, employer-sponsored on-the-job training, and industry-recognized credentialing examinations, including in the skilled trades, health care, commercial transportation, and cybersecurity.",
						},
						{
							Header: "Testing reimbursement",
							Text:   "The Secretary shall reimburse the cost of up to three licensing or certification tests per veteran, including the cost of a single retake of a failed test.",
						},
						{
							Header: "Employer standards",
							Text:   "An employer-sponsored program is eligible only if it pays a progressively increasing wage and reports completion and retention outcomes annually.",
						},
					},
				},
				{
					Header: "Survivors and dependents",
					Subs: []subsection{
						{
							Header: "Fry Scholarship",
							Text:   "The Marine Gunnery Sergeant John David Fry Scholarship is extended to the surviving spouse of a member who dies of a service-connected disability within 10 years after separation from the Armed Forces.",
						},
						{
							Header: "Dependents' assistance",
							Text:   "The monthly rate of educational assistance under chapter 35 of title 38, United States Code, is increased by 20 percent and indexed annually to the Consumer Price Index for All Urban Consumers.",
						},
					},
				},
				{
					Header: "Transition assistance",
					Subs: []subsection{
						{
							Header: "Counseling",
							Text:   "The Transition Assistance Program shall include a mandatory one-on-one counseling session conducted not later than 180 days before separation, covering education benefits, disability claims, and employment services.",
						},
						{
							Header: "Follow-up",
							Text:   "The Secretary shall contact each separating member at 90, 180, and 365 days after separation and shall report the contact rate to Congress each fiscal year.",
						},
					},
				},
				{
					Header: "Rural access",
					Subs: []subsection{
						{
							Header: "Grants",
							Text:   "There is authorized to be appropriated $250,000,000 for grants to community colleges and land-grant institutions serving veterans in rural counties, including for broadband-enabled distance learning and for on-campus veteran resource centers.",
						},
						{
							Header: "Priority",
							Text:   "Priority shall be given to institutions in counties in which the nearest Department of Veterans Affairs regional office is more than 100 miles away.",
						},
					},
				},
				{
					Header: "Oversight of predatory recruiting",
					Subs: []subsection{
						{
							Header: "Revenue limitation",
							Text:   "A covered institution that derives more than 85 percent of its revenue from Federal educational assistance, including assistance under chapter 33 of title 38, United States Code, may not enroll new beneficiaries until it comes into compliance.",
						},
						{
							Header: "Recruiting practices",
							Text:   "The Secretary shall suspend approval of a covered institution that engages in deceptive recruiting, misrepresents the transferability of credit, or pays incentive compensation based on enrollment.",
						},
						{
							Header: "Complaint tracking",
							Text:   "The Secretary shall maintain a public complaint tracking system and shall publish the number and disposition of complaints by institution each year.",
						},
					},
				},
				{
					Header: "Authorization of appropriations",
					Text:   "Amounts authorized by this Act are offset by extending existing fee authority under section 3729 of title 38, United States Code, through fiscal year 2034, and no additional appropriation is authorized except as expressly provided.",
				},
				{
					Header: "Reports",
					Text:   "The Comptroller General shall submit to the Committees on Veterans' Affairs of the Senate and the House of Representatives, not later than 2 years after the date of enactment, a report on the completion rates and post-program earnings of veterans using entitlement for apprenticeships and credentialing under section 4.",
				},
			}),
		PolicyArea:     "Armed Forces and National Security",
		Sponsor:        "Sen. Moran, Jerry [R-KS]",
		SponsorParty:   "R",
		IdeologyScore:  0.05,
		IntroducedDate: day(2025, time.March, 27),
		UpdatedAt:      day(2025, time.May, 7),
	}
}

func childrenOnline() models.Bill {
	const name = "Protecting Children Online Safety Act"
	return models.Bill{
		ID:      "hr-1986",
		Number:  "H.R. 1986",
		Title:   name,
		Chamber: models.ChamberHouse,
		Status:  "Ordered to be Reported by the Committee on Energy and Commerce",
		Summary: "Enhances online safety for children by strengthening privacy protections and increasing platform accountability.",
		FullText: billXML("H.R. 1986", name, models.ChamberHouse,
			"To protect the safety and privacy of minors online, and for other purposes.",
			[]section{
				shortTitle(name),
				definitions("In this Act:", [][2]string{
					{"covered platform", "an online service, application, or mobile application that connects users or serves algorithmically ranked content and that is reasonably likely to be accessed by a minor."},
					{"minor", "an individual under 17 years of age."},
					{"known minor", "a user whom the covered platform knows, or has reason to know based on information available to it, is a minor."},
					{"Commission", "the Federal Trade Commission."},
				}),
				{
					Header: "Duty of care",
					Subs: []subsection{
						{
							Header: "In general",
							Text:   "A covered platform shall exercise reasonable care in the design and operation of its products to prevent and mitigate reasonably foreseeable harms to minors, including compulsive usage, sexual exploitation, financial exploitation, and the promotion of self-harm, disordered eating, and substance abuse.",
						},
						{
							Header: "Limitation",
							Text:   "Subsection (a) shall not be construed to require a covered platform to prevent a minor from deliberately seeking content, or from receiving resources for the prevention or mitigation of the harms described in that subsection.",
						},
					},
				},
				{
					Header: "Default settings",
					Subs: []subsection{
						{
							Header: "Engagement features",
							Text:   "For a known minor account, a covered platform shall by default disable personalized recommendation systems, infinite scroll, autoplay, and streak-based engagement mechanics, and shall provide a clearly labelled control to re-enable each feature.",
						},
						{
							Header: "Contact and location",
							Text:   "Geolocation sharing and direct messaging from unconnected adult accounts shall be off by default for a known minor account, and a covered platform may not recommend a known minor account to an unconnected adult.",
						},
						{
							Header: "Quiet hours",
							Text:   "Notifications to a known minor account shall be suppressed between 12:00 a.m. and 6:00 a.m. in the user's local time unless the guardian elects otherwise.",
						},
					},
				},
				{
					Header: "Parental tools",
					Subs: []subsection{
						{
							Header: "Required tools",
							Text:   "Covered platforms shall provide guardians with tools to view privacy settings, restrict purchases and in-app spending, and set daily time limits, and shall notify the minor when a guardian control is active.",
						},
						{
							Header: "Reporting channel",
							Text:   "Covered platforms shall provide a clear and conspicuous channel for reporting harms to minors, and shall substantively respond within 10 days, or within 2 days in the case of a report involving imminent risk of physical harm.",
						},
					},
				},
				{
					Header: "Advertising and data",
					Subs: []subsection{
						{
							Header: "Targeted advertising",
							Text:   "Targeted advertising to a known minor on the basis of personal data is prohibited, except for contextual advertising based solely on the content the minor is then viewing.",
						},
						{
							Header: "Data minimisation",
							Text:   "Personal data of a known minor may not be sold, shared with a third party, or used to train a machine learning model except as strictly necessary to provide the service the minor requested.",
						},
						{
							Header: "Deletion",
							Text:   "A covered platform shall honour a request by a minor or a guardian to delete the minor's personal data within 30 days, and shall confirm the deletion in writing.",
						},
					},
				},
				{
					Header: "Age assurance",
					Subs: []subsection{
						{
							Header: "Rulemaking",
							Text:   "The Commission shall issue rules for privacy-preserving age assurance, which shall permit a covered platform to satisfy its obligations through inference from existing signals rather than through the collection of identity documents.",
						},
						{
							Header: "Retention limit",
							Text:   "A covered platform may not retain an identity document submitted for age assurance beyond the period necessary to complete verification, and in no case beyond 7 days.",
						},
					},
				},
				{
					Header: "Transparency and audit",
					Subs: []subsection{
						{
							Header: "Audits",
							Text:   "Covered platforms with more than 10,000,000 monthly active users in the United States shall publish an annual independent audit of reasonably foreseeable risks to minors, including the methodology and the aggregate results.",
						},
						{
							Header: "Researcher access",
							Text:   "Covered platforms described in subsection (a) shall provide vetted independent researchers with access to platform data necessary to study risks to minors, under a protocol approved by the Commission that protects user privacy and platform security.",
						},
					},
				},
				{
					Header: "Enforcement",
					Subs: []subsection{
						{
							Header: "Federal Trade Commission",
							Text:   "A violation of this Act shall be treated as an unfair or deceptive act or practice under section 5(a)(1) of the Federal Trade Commission Act, and the Commission shall enforce this Act in the same manner as that Act.",
						},
						{
							Header: "State enforcement",
							Text:   "The attorney general of a State may bring a civil action on behalf of the residents of that State for a violation of this Act, after providing written notice to the Commission.",
						},
						{
							Header: "Safe harbour",
							Text:   "A covered platform that adopts and adheres to a compliance program approved by the Commission shall be presumed to satisfy sections 3 and 4, subject to rebuttal by clear and convincing evidence.",
						},
					},
				},
				{
					Header: "Rules of construction",
					Text:   "Nothing in this Act shall be construed to require the general monitoring of user content, to require the retention of data that would not otherwise be retained, to authorize the removal of lawful speech by adults, or to preempt a State law that affords greater protection to minors.",
				},
				{
					Header: "Effective date",
					Text:   "This Act takes effect 18 months after the date of enactment, except that section 7 takes effect on the date of enactment for purposes of rulemaking.",
				},
			}),
		PolicyArea:     "Science, Technology, Communications",
		Sponsor:        "Rep. Bilirakis, Gus [R-FL]",
		SponsorParty:   "R",
		IdeologyScore:  -0.12,
		IntroducedDate: day(2025, time.March, 6),
		UpdatedAt:      day(2025, time.May, 6),
	}
}

func drugCosts() models.Bill {
	const name = "Lower Drug Costs for American Families Act"
	return models.Bill{
		ID:      "s-1025",
		Number:  "S. 1025",
		Title:   name,
		Chamber: models.ChamberSenate,
		Status:  "Read twice and referred to the Committee on Finance",
		Summary: "Empowers Medicare to negotiate drug prices and increases transparency to lower out-of-pocket costs for families.",
		FullText: billXML("S. 1025", name, models.ChamberSenate,
			"To lower prescription drug prices for beneficiaries under the Medicare program, and for other purposes.",
			[]section{
				shortTitle(name),
				definitions("In this Act:", [][2]string{
					{"Secretary", "the Secretary of Health and Human Services."},
					{"selected drug", "a drug or biological product selected for negotiation under part E of title XI of the Social Security Act."},
					{"maximum fair price", "the price negotiated by the Secretary for a selected drug under section 1194 of the Social Security Act."},
					{"pharmacy benefit manager", "an entity that administers prescription drug benefits on behalf of a group health plan or health insurance issuer."},
				}),
				{
					Header: "Expanded negotiation",
					Subs: []subsection{
						{
							Header: "Number of drugs",
							Text:   "The number of drugs selected for negotiation under part E of title XI of the Social Security Act is increased from 20 to 50 per year beginning in 2027, with selection based on total expenditures under parts B and D.",
						},
						{
							Header: "Eligibility timeline",
							Text:   "A drug becomes eligible for selection 7 years after approval under section 505(c) of the Federal Food, Drug, and Cosmetic Act for a small molecule drug, and 9 years after licensure under section 351(a) of the Public Health Service Act for a biological product.",
						},
						{
							Header: "Small biotech transition",
							Text:   "The small biotech exception is extended through 2031 for a manufacturer whose selected drug accounts for not more than 1 percent of total part D expenditures.",
						},
					},
				},
				{
					Header: "Negotiated prices in the commercial market",
					Subs: []subsection{
						{
							Header: "Election",
							Text:   "A group health plan or health insurance issuer offering group or individual coverage may elect to purchase a selected drug at the maximum fair price, and a manufacturer may not refuse to offer that price to an electing plan.",
						},
						{
							Header: "Administration",
							Text:   "The Secretary shall establish a mechanism for the reconciliation of payments between manufacturers, plans, and pharmacies, and shall not require a pharmacy to bear the cost of the discount before reimbursement.",
						},
					},
				},
				{
					Header: "Insulin and inhalers",
					Subs: []subsection{
						{
							Header: "Cost sharing",
							Text:   "Cost sharing for a month's supply of insulin is capped at $35, and for a covered inhaler at $35, for enrollees in Medicare part D and in group and individual market plans, without regard to whether the deductible has been satisfied.",
						},
						{
							Header: "Uninsured access",
							Text:   "The Secretary shall establish a program under which an uninsured individual may purchase insulin at not more than the maximum fair price at a participating pharmacy.",
						},
					},
				},
				{
					Header: "Out-of-pocket cap",
					Text:   "The annual out-of-pocket threshold under Medicare part D is reduced from $2,000 to $1,500, indexed thereafter to the Consumer Price Index for All Urban Consumers rather than to per capita part D spending, and the smoothing option under section 1860D-2(b)(2)(E) shall be offered by every plan.",
				},
				{
					Header: "Pharmacy benefit manager transparency",
					Subs: []subsection{
						{
							Header: "Spread pricing",
							Text:   "Spread pricing is prohibited in Medicaid managed care and in Medicare part D, and a pharmacy benefit manager shall be compensated only through a transparent flat fee unrelated to the list price of a drug.",
						},
						{
							Header: "Rebate pass-through",
							Text:   "A pharmacy benefit manager shall pass through 100 percent of rebates, fees, and other remuneration to the plan sponsor, and shall report aggregate rebate, fee, and net price data to the Secretary semiannually.",
						},
						{
							Header: "Pharmacy reimbursement",
							Text:   "A pharmacy benefit manager may not reimburse an unaffiliated pharmacy at a rate lower than the rate paid to a pharmacy it owns or controls for the same drug and quantity.",
						},
					},
				},
				{
					Header: "Generic and biosimilar competition",
					Subs: []subsection{
						{
							Header: "Unfair methods of competition",
							Text:   "Product hopping and the abuse of citizen petitions to delay generic entry are designated unfair methods of competition under section 5 of the Federal Trade Commission Act.",
						},
						{
							Header: "Pay-for-delay",
							Text:   "An agreement resolving patent litigation in which the holder of an approved application transfers value to a filer of an abbreviated application is presumptively anticompetitive, rebuttable by clear and convincing evidence.",
						},
						{
							Header: "Biosimilar interchangeability",
							Text:   "The Secretary shall streamline the demonstration of interchangeability for a biosimilar biological product and shall publish guidance within 1 year after the date of enactment.",
						},
					},
				},
				{
					Header: "Savings",
					Text:   "Amounts saved under this Act shall be credited to the Federal Supplementary Medical Insurance Trust Fund, and the Chief Actuary of the Centers for Medicare & Medicaid Services shall publish an annual accounting of those amounts.",
				},
				{
					Header: "Rules of construction",
					Text:   "Nothing in this Act shall be construed to authorize the Secretary to establish a national formulary, to deny coverage of a drug on the basis of a quality-adjusted life year metric, or to interfere with the clinical judgment of a treating physician.",
				},
			}),
		PolicyArea:     "Health",
		Sponsor:        "Sen. Bennet, Michael F. [D-CO]",
		SponsorParty:   "D",
		IdeologyScore:  -0.58,
		IntroducedDate: day(2025, time.March, 13),
		UpdatedAt:      day(2025, time.May, 6),
	}
}

func aiResearch() models.Bill {
	const name = "AI Research and Innovation Investment Act"
	return models.Bill{
		ID:      "hr-2890",
		Number:  "H.R. 2890",
		Title:   name,
		Chamber: models.ChamberHouse,
		Status:  "Referred to the Committee on Science, Space, and Technology",
		Summary: "Invests in AI research, workforce development, and public-private partnerships to maintain U.S. leadership in AI.",
		FullText: billXML("H.R. 2890", name, models.ChamberHouse,
			"To invest in artificial intelligence research and development, and for other purposes.",
			[]section{
				shortTitle(name),
				definitions("In this Act:", [][2]string{
					{"Director", "the Director of the National Science Foundation."},
					{"frontier model", "an artificial intelligence model trained using a quantity of computing power that exceeds the threshold established by the Director of the National Institute of Standards and Technology."},
					{"eligible researcher", "a researcher affiliated with an institution of higher education, a national laboratory, or a nonprofit research organization."},
				}),
				{
					Header: "National AI research resource",
					Subs: []subsection{
						{
							Header: "Establishment",
							Text:   "There is authorized to be appropriated $2,600,000,000 over 6 years to establish a shared national research infrastructure providing compute, curated datasets, and testbeds to eligible researchers, administered by the National Science Foundation.",
						},
						{
							Header: "Allocation",
							Text:   "Not less than 30 percent of compute allocated under this section shall be reserved for institutions that are not among the 25 largest recipients of Federal research funding.",
						},
						{
							Header: "Cost recovery",
							Text:   "The Director may recover the incremental cost of commercial use of the resource and shall reinvest recovered amounts in the resource.",
						},
					},
				},
				{
					Header: "Standards and evaluation",
					Subs: []subsection{
						{
							Header: "Voluntary standards",
							Text:   "The National Institute of Standards and Technology shall develop voluntary consensus standards for the evaluation of frontier model capabilities, including benchmarks for reliability, security, cyber-offensive capability, and misuse potential.",
						},
						{
							Header: "Testing program",
							Text:   "The Institute shall operate a testing program open to developers on a voluntary basis, and shall publish aggregate results without disclosing proprietary model weights or training data.",
						},
					},
				},
				{
					Header: "Workforce",
					Subs: []subsection{
						{
							Header: "Traineeships",
							Text:   "$900,000,000 is authorized for AI traineeships, community college certificate programs, and teacher professional development, with priority for programs serving first-generation college students.",
						},
						{
							Header: "Scholarship for service",
							Text:   "A scholarship-for-service program is established under which a graduate receives tuition support in exchange for a period of Federal service of equal duration in an artificial intelligence role.",
						},
					},
				},
				{
					Header: "Public-private partnerships",
					Text:   "The Secretary of Commerce shall establish not fewer than 8 regional artificial intelligence innovation institutes pairing universities with private partners, with a 30 percent non-Federal cost share, and shall ensure that no fewer than 3 are located in States that receive less than 1 percent of Federal research funding.",
				},
				{
					Header: "Applied research priorities",
					Text:   "Priority is given to applications in materials discovery, electric grid optimization, drug development, agricultural productivity, wildfire prediction, and the delivery of government services, and the Director shall report annually on obligations by priority area.",
				},
				{
					Header: "Research security",
					Subs: []subsection{
						{
							Header: "Security plans",
							Text:   "A recipient of funds under this Act shall implement a research security plan and shall disclose participation in a foreign talent recruitment program by any covered individual on the award.",
						},
						{
							Header: "Limitation",
							Text:   "Nothing in this Act authorizes a licensing regime for the development, training, or release of an artificial intelligence model, or the imposition of liability on a developer for the lawful use of a model by a third party.",
						},
					},
				},
				{
					Header: "Energy and infrastructure",
					Text:   "The Secretary of Energy shall assess the electricity, water, and transmission requirements of federally supported artificial intelligence computing facilities, and shall report to Congress on siting options that do not raise retail rates for existing customers.",
				},
				{
					Header: "Oversight",
					Text:   "The Comptroller General shall report to Congress every 2 years on the return on Federal artificial intelligence research investment, including publications, patents, spin-out formation, and the share of compute delivered to institutions described in section 3(b).",
				},
			}),
		PolicyArea:     "Science, Technology, Communications",
		Sponsor:        "Rep. Obernolte, Jay [R-CA]",
		SponsorParty:   "R",
		IdeologyScore:  -0.18,
		IntroducedDate: day(2025, time.April, 15),
		UpdatedAt:      day(2025, time.May, 5),
	}
}

func cybersecurity() models.Bill {
	const name = "National Cybersecurity Resilience Act"
	return models.Bill{
		ID:      "s-875",
		Number:  "S. 875",
		Title:   name,
		Chamber: models.ChamberSenate,
		Status:  "Reported by the Committee on Homeland Security and Governmental Affairs",
		Summary: "Strengthens critical infrastructure protections and enhances federal coordination on cybersecurity threats.",
		FullText: billXML("S. 875", name, models.ChamberSenate,
			"To strengthen the cybersecurity of critical infrastructure and Federal networks, and for other purposes.",
			[]section{
				shortTitle(name),
				definitions("In this Act:", [][2]string{
					{"Agency", "the Cybersecurity and Infrastructure Security Agency."},
					{"covered entity", "an owner or operator of critical infrastructure in a sector designated under Presidential Policy Directive 21 that meets the size thresholds established by the Director of the Agency."},
					{"substantial cyber incident", "an incident that leads to substantial loss of confidentiality, integrity, or availability of an information system, or a serious impact on the safety and resiliency of operational systems."},
				}),
				{
					Header: "Incident reporting",
					Subs: []subsection{
						{
							Header: "Deadlines",
							Text:   "A covered entity shall report a substantial cyber incident to the Agency within 72 hours after forming a reasonable belief that the incident occurred, and shall report a ransom payment within 24 hours after the payment is made.",
						},
						{
							Header: "Protections",
							Text:   "A report submitted under this section is exempt from disclosure under section 552 of title 5, United States Code, may not be used as direct evidence in an enforcement action against the reporting entity, and does not waive any applicable privilege.",
						},
						{
							Header: "Harmonisation",
							Text:   "The Director shall enter into agreements with other Federal regulators so that a single report satisfies substantially similar reporting obligations under other Federal law.",
						},
					},
				},
				{
					Header: "Minimum practices",
					Subs: []subsection{
						{
							Header: "Standards",
							Text:   "Sector risk management agencies shall establish minimum cybersecurity practices for covered entities, including phishing-resistant multifactor authentication, network segmentation between information and operational technology, tested offline backups, and a documented recovery time objective.",
						},
						{
							Header: "Equivalence",
							Text:   "A covered entity that demonstrates conformance with a recognised framework, including the NIST Cybersecurity Framework, shall be deemed to satisfy subsection (a).",
						},
					},
				},
				{
					Header: "Federal networks",
					Subs: []subsection{
						{
							Header: "Zero trust",
							Text:   "Federal civilian executive branch agencies shall complete the zero trust architecture milestones established by the Director of the Office of Management and Budget within 3 years after the date of enactment.",
						},
						{
							Header: "Asset inventory",
							Text:   "The Federal Chief Information Security Officer shall maintain a continuous inventory of internet-facing assets across the civilian enterprise and shall remediate a known exploited vulnerability within 15 days after cataloguing.",
						},
						{
							Header: "Procurement",
							Text:   "A contract for software delivered to a Federal agency shall require a software bill of materials and attestation of secure development practices.",
						},
					},
				},
				{
					Header: "State, local, and rural support",
					Subs: []subsection{
						{
							Header: "Grants",
							Text:   "$1,200,000,000 is authorized for the State and Local Cybersecurity Grant Program, with a set-aside of not less than 25 percent for rural water systems, rural hospitals, and school districts.",
						},
						{
							Header: "Shared services",
							Text:   "The Agency shall offer no-cost vulnerability scanning, endpoint detection, and incident response retainers to entities with fewer than 500 employees.",
						},
					},
				},
				{
					Header: "Workforce",
					Text:   "The Cyber Service Academy scholarship program is expanded to 1,500 participants annually, agencies are granted direct hire authority for cybersecurity positions through 2032, and the Director shall establish a rotational program allowing Federal cybersecurity personnel to serve temporarily with State and local governments.",
				},
				{
					Header: "International and supply chain",
					Subs: []subsection{
						{
							Header: "Vendor list",
							Text:   "The Secretary shall maintain and publish a list of hardware and software vendors that present an unacceptable supply chain risk, together with the basis for each listing and a process for delisting.",
						},
						{
							Header: "Coordination",
							Text:   "The Secretary shall coordinate joint attribution statements and sanctions recommendations with allied governments, and shall report to Congress on the disposition of each recommendation.",
						},
					},
				},
				{
					Header: "Sunset",
					Text:   "The reporting requirements under section 3 expire 8 years after the date of enactment unless reauthorized, and the Comptroller General shall submit a report evaluating their effectiveness not later than 6 years after the date of enactment.",
				},
			}),
		PolicyArea:     "Science, Technology, Communications",
		Sponsor:        "Sen. Peters, Gary C. [D-MI]",
		SponsorParty:   "D",
		IdeologyScore:  0.08,
		IntroducedDate: day(2025, time.March, 5),
		UpdatedAt:      day(2025, time.May, 5),
	}
}

func affordableHousing() models.Bill {
	const name = "Affordable Housing Supply Act"
	return models.Bill{
		ID:      "hr-1567",
		Number:  "H.R. 1567",
		Title:   name,
		Chamber: models.ChamberHouse,
		Status:  "Referred to the Committee on Financial Services",
		Summary: "Incentivizes the construction of affordable housing and streamlines local permitting processes.",
		FullText: billXML("H.R. 1567", name, models.ChamberHouse,
			"To increase the supply of affordable housing, and for other purposes.",
			[]section{
				shortTitle(name),
				definitions("In this Act:", [][2]string{
					{"Secretary", "the Secretary of Housing and Urban Development."},
					{"area median income", "the median income for the area as determined annually by the Secretary."},
					{"qualifying project", "a residential project in which not less than 20 percent of units are affordable to households at or below 60 percent of area median income."},
				}),
				{
					Header: "Low-income housing tax credit",
					Subs: []subsection{
						{
							Header: "State ceiling",
							Text:   "The State housing credit ceiling under section 42(h)(3) of the Internal Revenue Code of 1986 is increased by 50 percent and indexed for inflation.",
						},
						{
							Header: "Bond financing threshold",
							Text:   "The private activity bond financing threshold for the 4 percent credit is reduced from 50 percent to 25 percent of aggregate basis.",
						},
						{
							Header: "Basis boost",
							Text:   "A 30 percent basis boost is made available for projects in rural areas and on Tribal land at the discretion of the State housing credit agency.",
						},
					},
				},
				{
					Header: "Neighborhood homes credit",
					Text:   "A credit is established for the construction or substantial rehabilitation of owner-occupied homes in distressed census tracts, limited to the lesser of the gap between development cost and appraised value or 40 percent of that value, and recaptured on a sale within 5 years other than to a qualifying owner-occupant.",
				},
				{
					Header: "Unlocking local supply",
					Subs: []subsection{
						{
							Header: "Housing supply plans",
							Text:   "A jurisdiction receiving Community Development Block Grant funds shall submit a housing supply plan addressing minimum lot sizes, parking mandates, accessory dwelling units, and multifamily zoning within one-half mile of fixed-route transit.",
						},
						{
							Header: "Competitive grants",
							Text:   "$3,000,000,000 is authorized for competitive grants to jurisdictions that adopt by-right approval for qualifying projects and reduce median permitting time below 90 days.",
						},
						{
							Header: "Limitation",
							Text:   "Nothing in this section authorizes the Secretary to require a jurisdiction to adopt a particular zoning ordinance, or to condition the receipt of formula funds on a change in local land use law.",
						},
					},
				},
				{
					Header: "Housing trust fund",
					Text:   "The Housing Trust Fund is capitalized with an additional $4,000,000,000 for the construction of rental housing affordable to extremely low-income households, of which not less than 10 percent shall be reserved for permanent supportive housing.",
				},
				{
					Header: "Vouchers and stability",
					Subs: []subsection{
						{
							Header: "New vouchers",
							Text:   "An additional 250,000 housing choice vouchers are authorized, phased in over 5 years, with landlord incentive payments and a damage mitigation fund.",
						},
						{
							Header: "Right to counsel pilot",
							Text:   "A Federal right to counsel pilot program is established in eviction proceedings in 20 jurisdictions, and the Secretary shall report on eviction filing and judgment rates in participating jurisdictions.",
						},
					},
				},
				{
					Header: "Manufactured and modular housing",
					Text:   "The Secretary shall update the manufactured housing construction and safety standards to permit multi-unit and duplex configurations and modular construction methods, and shall expand title I loan limits and indexing for personal property loans.",
				},
				{
					Header: "Federal land",
					Text:   "Surplus Federal land may be conveyed at a discount from fair market value to a public housing agency, local government, or nonprofit developer for a project in which at least 50 percent of units are affordable at 60 percent of area median income, subject to a 40-year use restriction recorded against the property.",
				},
				{
					Header: "Reports",
					Text:   "The Secretary shall report annually on units placed in service by income band, median permitting times in grant recipient jurisdictions, and the utilization rate of vouchers authorized under section 7.",
				},
			}),
		PolicyArea:     "Housing and Community Development",
		Sponsor:        "Rep. Waters, Maxine [D-CA]",
		SponsorParty:   "D",
		IdeologyScore:  -0.45,
		IntroducedDate: day(2025, time.February, 25),
		UpdatedAt:      day(2025, time.May, 4),
	}
}

func smallBusinessTax() models.Bill {
	const name = "Small Business Tax Relief and Simplification Act"
	return models.Bill{
		ID:      "s-742",
		Number:  "S. 742",
		Title:   name,
		Chamber: models.ChamberSenate,
		Status:  "Read twice and referred to the Committee on Finance",
		Summary: "Makes the qualified business income deduction permanent, restores immediate expensing, and simplifies filing for small employers.",
		FullText: billXML("S. 742", name, models.ChamberSenate,
			"To provide tax relief and filing simplification for small businesses, and for other purposes.",
			[]section{
				shortTitle(name),
				definitions("In this Act:", [][2]string{
					{"Secretary", "the Secretary of the Treasury or the Secretary's delegate."},
					{"small employer", "an employer that employed an average of fewer than 50 full-time equivalent employees during the preceding calendar year."},
					{"qualified business income", "income described in section 199A(c) of the Internal Revenue Code of 1986."},
				}),
				{
					Header: "Qualified business income",
					Subs: []subsection{
						{
							Header: "Permanence",
							Text:   "The 20 percent deduction for qualified business income under section 199A of the Internal Revenue Code of 1986 is made permanent by striking the termination date.",
						},
						{
							Header: "Thresholds",
							Text:   "The phase-in thresholds under section 199A(e)(2) are increased to $500,000 for individual filers and $1,000,000 for joint filers, indexed for inflation after 2026.",
						},
					},
				},
				{
					Header: "Expensing",
					Subs: []subsection{
						{
							Header: "Bonus depreciation",
							Text:   "Full expensing of qualified property under section 168(k) of the Internal Revenue Code of 1986 is restored and made permanent for property placed in service after December 31, 2025.",
						},
						{
							Header: "Research expenditures",
							Text:   "Domestic research and experimental expenditures may again be deducted in the taxable year incurred rather than amortized over 5 years, with a transition election for amounts previously capitalized.",
						},
						{
							Header: "Section 179",
							Text:   "The dollar limitation under section 179 is increased to $2,500,000 with a phase-out beginning at $4,000,000 of property placed in service.",
						},
					},
				},
				{
					Header: "Simplification",
					Subs: []subsection{
						{
							Header: "Cash method",
							Text:   "The average annual gross receipts threshold for use of the cash method of accounting under section 448(c) is raised to $50,000,000, indexed for inflation.",
						},
						{
							Header: "Single simplified return",
							Text:   "The Secretary shall publish a single simplified return for small employers not later than 2 years after the date of enactment, and may not require paper filing of any information return by a small employer.",
						},
						{
							Header: "Estimated payments",
							Text:   "A small employer may elect to make estimated tax payments on a semiannual rather than quarterly basis if its liability for the preceding year was less than $25,000.",
						},
					},
				},
				{
					Header: "Startup costs",
					Text:   "The deduction for startup and organizational expenditures under sections 195 and 248 of the Internal Revenue Code of 1986 is increased from $5,000 to $50,000, with the phase-out threshold increased to $150,000.",
				},
				{
					Header: "Compliance relief",
					Subs: []subsection{
						{
							Header: "Beneficial ownership",
							Text:   "Beneficial ownership reporting deadlines under the Corporate Transparency Act are extended by 2 years, and the Secretary shall conduct a public education campaign before enforcement resumes.",
						},
						{
							Header: "First-time penalty waiver",
							Text:   "A first-time penalty waiver is established for a good faith error by a small employer on an employment tax or information return, available once every 3 years.",
						},
					},
				},
				{
					Header: "Taxpayer service",
					Text:   "The Internal Revenue Service shall answer not less than 85 percent of telephone calls from small employers during the filing season, shall offer callback scheduling, and shall report monthly on the level of service achieved.",
				},
				{
					Header: "Offset",
					Text:   "The Joint Committee on Taxation shall report the revenue effect of this Act, and any amount not offset shall be recorded on the pay-as-you-go scorecard established under the Statutory Pay-As-You-Go Act of 2010.",
				},
			}),
		PolicyArea:     "Taxation",
		Sponsor:        "Sen. Ernst, Joni [R-IA]",
		SponsorParty:   "R",
		IdeologyScore:  0.7,
		IntroducedDate: day(2025, time.February, 26),
		UpdatedAt:      day(2025, time.May, 2),
	}
}

func farmland() models.Bill {
	const name = "Farmland Preservation and Rural Investment Act"
	return models.Bill{
		ID:      "hr-4102",
		Number:  "H.R. 4102",
		Title:   name,
		Chamber: models.ChamberHouse,
		Status:  "Referred to the Committee on Agriculture",
		Summary: "Protects working farmland from conversion, expands conservation easements, and funds rural broadband and processing capacity.",
		FullText: billXML("H.R. 4102", name, models.ChamberHouse,
			"To preserve working farmland, expand rural infrastructure, and for other purposes.",
			[]section{
				shortTitle(name),
				definitions("In this Act:", [][2]string{
					{"Secretary", "the Secretary of Agriculture."},
					{"beginning farmer", "a producer who has operated a farm or ranch for not more than 10 years."},
					{"country of concern", "a country identified as a foreign adversary under section 791.2 of title 15, Code of Federal Regulations."},
					{"working farmland", "land that has been in agricultural production in at least 3 of the 5 preceding years."},
				}),
				{
					Header: "Agricultural land easements",
					Subs: []subsection{
						{
							Header: "Funding",
							Text:   "Funding for the Agricultural Conservation Easement Program is increased to $1,000,000,000 annually through fiscal year 2032.",
						},
						{
							Header: "Priority",
							Text:   "Priority shall be given to working farmland in counties experiencing the highest rate of conversion to non-agricultural use, as measured by the National Resources Inventory.",
						},
						{
							Header: "Term",
							Text:   "An easement acquired under this section shall be perpetual, except that the Secretary may accept a term easement of not less than 30 years where State law prohibits a perpetual interest.",
						},
					},
				},
				{
					Header: "Beginning farmers",
					Subs: []subsection{
						{
							Header: "Down payment loans",
							Text:   "The down payment loan program is expanded, with the interest rate reduced to 1.5 percent for beginning, veteran, and socially disadvantaged producers and the maximum purchase price increased to $1,000,000.",
						},
						{
							Header: "Transition credit",
							Text:   "A tax credit is provided to a landowner who sells or leases working farmland to a beginning farmer under an agreement of not less than 5 years.",
						},
					},
				},
				{
					Header: "Foreign ownership",
					Subs: []subsection{
						{
							Header: "Prohibition",
							Text:   "The acquisition of agricultural land by an entity owned or controlled by a country of concern is prohibited, and an existing holding shall be divested within 2 years after the date of enactment.",
						},
						{
							Header: "Reporting",
							Text:   "Reporting under the Agricultural Foreign Investment Disclosure Act of 1978 is modernized to require electronic filing, and civil penalties for nondisclosure are increased to 25 percent of the fair market value of the interest.",
						},
					},
				},
				{
					Header: "Processing capacity",
					Text:   "$600,000,000 is authorized for grants and guaranteed loans to small and mid-sized meat and poultry processors, dairy processors, and grain handling facilities, with a preference for producer-owned cooperatives and a limit of $25,000,000 per recipient.",
				},
				{
					Header: "Rural broadband",
					Subs: []subsection{
						{
							Header: "Reauthorization",
							Text:   "The ReConnect Program is reauthorized at $2,000,000,000 annually through fiscal year 2031 with a minimum service standard of 100 megabits per second downstream and 20 megabits per second upstream.",
						},
						{
							Header: "Open access",
							Text:   "A preference shall be given to applications proposing open-access middle mile facilities, and a recipient shall offer a low-cost service option to eligible households.",
						},
					},
				},
				{
					Header: "Conservation practices",
					Text:   "Cost-share rates are increased to 90 percent for cover crops, precision nutrient management, and irrigation efficiency for producers operating fewer than 1,000 acres, and a voluntary soil carbon measurement pilot is established in not fewer than 10 States.",
				},
				{
					Header: "Report",
					Text:   "The Secretary shall report annually on acres of working farmland converted to non-agricultural use, easements enrolled by State, processing capacity added by species and region, and broadband locations served.",
				},
			}),
		PolicyArea:     "Agriculture and Food",
		Sponsor:        "Rep. Craig, Angie [D-MN]",
		SponsorParty:   "D",
		IdeologyScore:  -0.1,
		IntroducedDate: day(2025, time.April, 8),
		UpdatedAt:      day(2025, time.May, 1),
	}
}

func elections() models.Bill {
	const name = "Election Integrity and Voter Access Act"
	return models.Bill{
		ID:      "s-690",
		Number:  "S. 690",
		Title:   name,
		Chamber: models.ChamberSenate,
		Status:  "Read twice and referred to the Committee on Rules and Administration",
		Summary: "Pairs new voter list maintenance and audit requirements with minimum standards for early voting and mail ballot cure.",
		FullText: billXML("S. 690", name, models.ChamberSenate,
			"To improve the administration of Federal elections, and for other purposes.",
			[]section{
				shortTitle(name),
				definitions("In this Act:", [][2]string{
					{"Commission", "the Election Assistance Commission."},
					{"Federal contest", "an election for the office of President, Vice President, Senator, or Representative in, or Delegate or Resident Commissioner to, the Congress."},
					{"risk-limiting audit", "a post-election audit that has a predetermined chance of correcting an incorrect reported outcome."},
				}),
				{
					Header: "List maintenance",
					Subs: []subsection{
						{
							Header: "Annual maintenance",
							Text:   "Each State shall conduct annual list maintenance using death records, State motor vehicle records, change-of-address data, and interstate cross-checks, and shall publish the number of registrations removed by category.",
						},
						{
							Header: "Limitation",
							Text:   "A registrant may not be removed from the rolls solely for a failure to vote, and a removal within 90 days before a Federal contest is prohibited except upon the request of the registrant or on the basis of a death record.",
						},
					},
				},
				{
					Header: "Audits",
					Subs: []subsection{
						{
							Header: "Requirement",
							Text:   "Each State shall conduct a risk-limiting audit of Federal contests before certification, using a voter-verifiable paper record for every ballot cast.",
						},
						{
							Header: "Observation",
							Text:   "An audit shall be open to observation by representatives of each political party and by the public, and the risk limit and results shall be published.",
						},
					},
				},
				{
					Header: "Equipment security",
					Text:   "Voting systems used in a Federal contest shall be certified under the most recent Voluntary Voting System Guidelines, may not be connected to the internet or contain a wireless modem, and shall be subject to chain-of-custody logging, tamper-evident seals, and post-election logic and accuracy testing.",
				},
				{
					Header: "Minimum access standards",
					Subs: []subsection{
						{
							Header: "Early voting",
							Text:   "Each State shall provide not fewer than 10 days of early in-person voting for a Federal contest, including at least one weekend day, with locations open not fewer than 8 hours per day.",
						},
						{
							Header: "Mail ballot cure",
							Text:   "A voter whose mail ballot is rejected for a signature discrepancy or other technical defect shall receive notice by the fastest available means and an opportunity to cure not later than 5 days after the election.",
						},
						{
							Header: "Online registration",
							Text:   "Online voter registration shall be available in every State, with a paper alternative and a public accessibility standard.",
						},
					},
				},
				{
					Header: "Identification",
					Text:   "A State that requires photo identification to vote shall provide such identification at no cost, shall accept a signed affidavit of identity together with a provisional ballot from a voter who lacks it, and shall count that provisional ballot if the affidavit is not successfully challenged within 7 days.",
				},
				{
					Header: "Election administration funding",
					Text:   "$800,000,000 is authorized for the Commission for equipment replacement, cybersecurity, ballot tracking, and poll worker recruitment and training, distributed by formula with a small-State minimum of one-half of 1 percent.",
				},
				{
					Header: "Protection of election officials",
					Text:   "Threatening an election official or interfering with the transmission or certification of election results is subject to enhanced Federal penalties, and the Attorney General shall maintain a task force to receive and investigate such threats.",
				},
				{
					Header: "Effective date",
					Text:   "This Act applies to the first Federal general election occurring more than 1 year after the date of enactment, and the Commission shall issue implementing guidance within 180 days.",
				},
			}),
		PolicyArea:     "Government Operations and Politics",
		Sponsor:        "Sen. Collins, Susan M. [R-ME]",
		SponsorParty:   "R",
		IdeologyScore:  0.3,
		IntroducedDate: day(2025, time.February, 20),
		UpdatedAt:      day(2025, time.April, 30),
	}
}

func cleanWater() models.Bill {
	const name = "Clean Water Infrastructure Modernization Act"
	return models.Bill{
		ID:          "hr-3355",
		Number:      "H.R. 3355",
		Title:       name,
		Chamber:     models.ChamberHouse,
		Status:      "Passed House, referred to the Committee on Environment and Public Works",
		TextVersion: "Engrossed in House",
		Summary:     "Recapitalizes state revolving funds, accelerates lead service line replacement, and sets deadlines for PFAS remediation.",
		FullText: billXML("H.R. 3355", name, models.ChamberHouse,
			"To modernize the drinking water and wastewater infrastructure of the United States, and for other purposes.",
			[]section{
				shortTitle(name),
				definitions("In this Act:", [][2]string{
					{"Administrator", "the Administrator of the Environmental Protection Agency."},
					{"disadvantaged community", "a community meeting the affordability criteria established by a State under section 1452(d) of the Safe Drinking Water Act."},
					{"lead service line", "a portion of pipe made of lead that connects a water main to a building inlet, including a lead gooseneck or pigtail."},
					{"passive receiver", "a public water system, publicly owned treatment works, or airport that received a perfluoroalkyl or polyfluoroalkyl substance it did not manufacture."},
				}),
				{
					Header: "State revolving funds",
					Subs: []subsection{
						{
							Header: "Reauthorization",
							Text:   "The Clean Water and Drinking Water State Revolving Funds are reauthorized at a combined $9,000,000,000 annually through fiscal year 2031.",
						},
						{
							Header: "Principal forgiveness",
							Text:   "Not less than 25 percent of each State's capitalization grant shall be provided as principal forgiveness or negative interest loans to disadvantaged communities.",
						},
						{
							Header: "Technical assistance",
							Text:   "Not less than 2 percent shall be reserved for technical assistance to systems serving fewer than 10,000 people, including asset management and rate-setting support.",
						},
					},
				},
				{
					Header: "Lead service lines",
					Subs: []subsection{
						{
							Header: "Inventory and replacement",
							Text:   "A public water system shall complete a full inventory of service line materials within 2 years after the date of enactment and shall replace all lead service lines within 10 years.",
						},
						{
							Header: "Funding and cost allocation",
							Text:   "$5,000,000,000 is authorized for replacement, and a system may not charge a customer for the private-side portion of a replacement funded under this section.",
						},
						{
							Header: "Notification",
							Text:   "A system shall notify each affected customer of the material of the service line serving the property, and shall provide filters during and for 6 months after a replacement.",
						},
					},
				},
				{
					Header: "Perfluoroalkyl and polyfluoroalkyl substances",
					Subs: []subsection{
						{
							Header: "Standards",
							Text:   "The Administrator shall establish enforceable national primary drinking water regulations for additional perfluoroalkyl and polyfluoroalkyl substances within 3 years after the date of enactment.",
						},
						{
							Header: "Treatment grants",
							Text:   "The Administrator shall provide treatment grants to small and rural systems, and shall not require a system to begin capital construction before grant funds are available.",
						},
						{
							Header: "Liability",
							Text:   "A passive receiver is not liable under section 107 of the Comprehensive Environmental Response, Compensation, and Liability Act of 1980 for a release of a perfluoroalkyl or polyfluoroalkyl substance, except in a case of gross negligence.",
						},
					},
				},
				{
					Header: "Stormwater and resilience",
					Text:   "A competitive grant program is established for green infrastructure, combined sewer overflow abatement, and flood resilience at wastewater treatment plants, with a Federal share of 80 percent for a disadvantaged community and 55 percent otherwise.",
				},
				{
					Header: "Workforce",
					Text:   "$100,000,000 is authorized for water operator apprenticeships, certification, and reciprocity across States, addressing the projected retirement of one-third of the operator workforce within 10 years.",
				},
				{
					Header: "Affordability",
					Text:   "The Low Income Household Water Assistance Program is made permanent and authorized at $1,000,000,000 annually, and a system receiving assistance under this Act shall report annually on shutoff and reconnection practices.",
				},
				{
					Header: "Buy America",
					Text:   "Iron, steel, and manufactured products used in a project assisted under this Act shall be produced in the United States, subject to the waiver authority in section 70914 of the Infrastructure Investment and Jobs Act, and each waiver shall be published for public comment before it is granted.",
				},
			}),
		PolicyArea:     "Water Resources Development",
		Sponsor:        "Rep. Napolitano, Grace F. [D-CA]",
		SponsorParty:   "D",
		IdeologyScore:  -0.35,
		IntroducedDate: day(2025, time.January, 30),
		UpdatedAt:      day(2025, time.April, 29),
	}
}
