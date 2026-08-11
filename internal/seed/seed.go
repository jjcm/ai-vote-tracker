// Package seed provides the offline bill corpus used when no Congress.gov API
// key is configured. The bills are realistic composites of recent House and
// Senate legislation, written to give the models enough statutory detail to
// reason about.
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
		{
			ID:      "s-1264",
			Number:  "S. 1264",
			Title:   "Border Security and Asylum Reform Act of 2025",
			Chamber: models.ChamberSenate,
			Status:  "Read twice and referred to the Committee on Homeland Security",
			Summary: "This bill strengthens border security measures, expands expedited removal authority, and reforms the asylum process to improve efficiency and integrity while ensuring protections for individuals with valid claims.",
			FullText: `SECTION 1. SHORT TITLE.
This Act may be cited as the "Border Security and Asylum Reform Act of 2025".

SEC. 2. BORDER INFRASTRUCTURE AND TECHNOLOGY.
(a) There is authorized to be appropriated $8,400,000,000 for fiscal years 2026 through 2030 for physical barriers, autonomous surveillance towers, unattended ground sensors, and non-intrusive inspection systems at ports of entry.
(b) Not less than 15 percent of amounts appropriated shall be used for contraband detection at ports of entry.

SEC. 3. PERSONNEL.
The Secretary shall hire not fewer than 3,000 additional Border Patrol agents, 1,200 Customs and Border Protection officers, and 500 asylum officers, and shall establish retention bonuses for agents assigned to remote sectors.

SEC. 4. EXPEDITED REMOVAL.
(a) Section 235(b)(1) of the Immigration and Nationality Act is amended to authorize expedited removal of an alien encountered anywhere in the United States who cannot establish continuous physical presence for two years.
(b) An alien subject to expedited removal who expresses a fear of persecution shall receive a credible fear interview not later than 7 days after such expression.

SEC. 5. CREDIBLE FEAR STANDARD.
The credible fear standard is raised from "significant possibility" to "more likely than not" that the alien can establish eligibility for asylum. An alien found not to have a credible fear may request review by an immigration judge within 48 hours.

SEC. 6. ASYLUM ADJUDICATION TIMELINES.
(a) The Attorney General shall complete initial asylum adjudications not later than 180 days after filing, and shall increase the immigration judge corps by 250 judges to meet that deadline.
(b) Work authorization for asylum applicants may not be issued earlier than 180 days after filing.

SEC. 7. PROTECTIONS.
Nothing in this Act shall be construed to limit access to counsel at no expense to the Government, to authorize the removal of an unaccompanied child without a hearing, or to abrogate obligations of the United States under the 1967 Protocol Relating to the Status of Refugees.

SEC. 8. REPORTING.
The Secretary shall submit to Congress an annual report on removals, credible fear determinations, and case backlogs, disaggregated by sector and nationality.`,
			PolicyArea:     "Immigration",
			Sponsor:        "Sen. Marshall, Roger [R-KS]",
			SponsorParty:   "R",
			IdeologyScore:  0.62,
			IntroducedDate: day(2025, time.April, 2),
			UpdatedAt:      day(2025, time.May, 8),
		},
		{
			ID:      "hr-3721",
			Number:  "H.R. 3721",
			Title:   "American Energy Independence Act",
			Chamber: models.ChamberHouse,
			Status:  "Referred to the Subcommittee on Energy and Mineral Resources",
			Summary: "Expands domestic energy production, accelerates permitting reforms, and streamlines energy infrastructure development.",
			FullText: `SECTION 1. SHORT TITLE.
This Act may be cited as the "American Energy Independence Act".

SEC. 2. FEDERAL LEASING.
(a) The Secretary of the Interior shall hold not fewer than four onshore oil and gas lease sales per year in each State with available Federal acreage, and shall resume quarterly offshore lease sales in the Gulf of America and Cook Inlet planning areas.
(b) A lease sale may not be cancelled or delayed except upon a written finding of a material defect in the sale notice.

SEC. 3. PERMITTING REFORM.
(a) Environmental reviews under the National Environmental Policy Act for covered energy projects shall be completed within 2 years for an environmental impact statement and 1 year for an environmental assessment, with a 300-page limit on the statement.
(b) Judicial challenges to a covered permit must be filed within 150 days of final agency action.
(c) A single lead agency shall issue one combined record of decision for each covered project.

SEC. 4. TRANSMISSION AND PIPELINES.
The Federal Energy Regulatory Commission shall act on an application for a natural gas pipeline certificate within 12 months, and is granted backstop siting authority for interstate transmission lines in designated national interest corridors.

SEC. 5. CRITICAL MINERALS.
Domestic mining projects for lithium, cobalt, nickel, copper, and rare earth elements are designated covered projects for purposes of section 3, and the Secretary shall complete a national assessment of critical mineral reserves within 18 months.

SEC. 6. EXPORTS.
Applications to export liquefied natural gas to non-free-trade-agreement countries shall be deemed consistent with the public interest unless the Secretary of Energy makes a contrary finding within 90 days.

SEC. 7. REPEALS.
The methane waste emissions charge established under section 136 of the Clean Air Act is repealed.`,
			PolicyArea:     "Energy",
			Sponsor:        "Rep. Westerman, Bruce [R-AR]",
			SponsorParty:   "R",
			IdeologyScore:  0.58,
			IntroducedDate: day(2025, time.March, 18),
			UpdatedAt:      day(2025, time.May, 7),
		},
		{
			ID:      "s-1198",
			Number:  "S. 1198",
			Title:   "Veterans' Education and Opportunity Act",
			Chamber: models.ChamberSenate,
			Status:  "Read twice and referred to the Committee on Veterans' Affairs",
			Summary: "Improves access to education, job training, and support services for veterans and their families.",
			FullText: `SECTION 1. SHORT TITLE.
This Act may be cited as the "Veterans' Education and Opportunity Act".

SEC. 2. RESTORATION OF ENTITLEMENT.
A veteran whose program of education is suspended or closed by the institution shall have the corresponding months of Post-9/11 GI Bill entitlement restored, and shall receive a housing stipend for up to 4 months while transferring.

SEC. 3. APPRENTICESHIP AND CREDENTIALING.
(a) Entitlement may be used for registered apprenticeships, employer-sponsored on-the-job training, and industry-recognized credentialing examinations, including in skilled trades, health care, and cybersecurity.
(b) The Secretary shall reimburse the cost of up to three licensing or certification tests per veteran.

SEC. 4. SURVIVORS AND DEPENDENTS.
The Marine Gunnery Sergeant John David Fry Scholarship is extended to the surviving spouse of a member who dies of a service-connected disability within 10 years of separation.

SEC. 5. TRANSITION ASSISTANCE.
The Transition Assistance Program shall include a mandatory one-on-one counseling session, and the Secretary shall contact each separating member at 90, 180, and 365 days after separation.

SEC. 6. RURAL ACCESS.
There is authorized to be appropriated $250,000,000 for grants to community colleges and land-grant institutions serving veterans in rural counties, including for broadband-enabled distance learning.

SEC. 7. OVERSIGHT OF PREDATORY RECRUITING.
An institution that derives more than 85 percent of revenue from Federal educational assistance may not enroll new beneficiaries until it comes into compliance.

SEC. 8. AUTHORIZATION.
Amounts authorized by this Act are offset by extending existing fee authority under section 3729 of title 38, United States Code.`,
			PolicyArea:     "Armed Forces and National Security",
			Sponsor:        "Sen. Moran, Jerry [R-KS]",
			SponsorParty:   "R",
			IdeologyScore:  0.05,
			IntroducedDate: day(2025, time.March, 27),
			UpdatedAt:      day(2025, time.May, 7),
		},
		{
			ID:      "hr-1986",
			Number:  "H.R. 1986",
			Title:   "Protecting Children Online Safety Act",
			Chamber: models.ChamberHouse,
			Status:  "Ordered to be Reported by the Committee on Energy and Commerce",
			Summary: "Enhances online safety for children by strengthening privacy protections and increasing platform accountability.",
			FullText: `SECTION 1. SHORT TITLE.
This Act may be cited as the "Protecting Children Online Safety Act".

SEC. 2. DUTY OF CARE.
A covered platform that is reasonably likely to be accessed by a minor shall exercise reasonable care in the design of its products to prevent and mitigate harms to minors, including compulsive usage, sexual exploitation, and the promotion of self-harm.

SEC. 3. DEFAULT SETTINGS.
(a) For a known minor account, a covered platform shall by default disable personalized recommendation systems, infinite scroll, autoplay, and streak-based engagement mechanics.
(b) Geolocation sharing and direct messaging from unconnected adult accounts shall be off by default.

SEC. 4. PARENTAL TOOLS.
Covered platforms shall provide guardians with tools to view privacy settings, restrict purchases, and set daily time limits, and shall provide a clear channel for reporting harms with a response within 10 days.

SEC. 5. ADVERTISING AND DATA.
Targeted advertising to a known minor is prohibited. Personal data of a known minor may not be sold or shared with a third party except as required to provide the service.

SEC. 6. AGE ASSURANCE.
The Commission shall issue rules for privacy-preserving age assurance. A platform may not retain identity documents submitted for age assurance beyond the period necessary to complete verification.

SEC. 7. TRANSPARENCY AND AUDIT.
Covered platforms with more than 10,000,000 monthly active users shall publish an annual independent audit of risks to minors and shall provide vetted researchers with access to platform data under a Commission-approved protocol.

SEC. 8. ENFORCEMENT.
A violation shall be treated as an unfair or deceptive act under section 5 of the Federal Trade Commission Act. State attorneys general may bring civil actions. Nothing in this Act shall be construed to require the general monitoring of user content or to authorize the removal of lawful speech by adults.`,
			PolicyArea:     "Science, Technology, Communications",
			Sponsor:        "Rep. Bilirakis, Gus [R-FL]",
			SponsorParty:   "R",
			IdeologyScore:  -0.12,
			IntroducedDate: day(2025, time.March, 6),
			UpdatedAt:      day(2025, time.May, 6),
		},
		{
			ID:      "s-1025",
			Number:  "S. 1025",
			Title:   "Lower Drug Costs for American Families Act",
			Chamber: models.ChamberSenate,
			Status:  "Read twice and referred to the Committee on Finance",
			Summary: "Empowers Medicare to negotiate drug prices and increases transparency to lower out-of-pocket costs for families.",
			FullText: `SECTION 1. SHORT TITLE.
This Act may be cited as the "Lower Drug Costs for American Families Act".

SEC. 2. EXPANDED NEGOTIATION.
(a) The number of drugs selected for negotiation under part E of title XI of the Social Security Act is increased from 20 to 50 per year beginning in 2027.
(b) A drug becomes eligible for selection 7 years after approval for small molecules and 9 years for biologics.

SEC. 3. NEGOTIATED PRICES IN THE COMMERCIAL MARKET.
A group health plan may elect to purchase a selected drug at the maximum fair price negotiated by the Secretary.

SEC. 4. INSULIN AND INHALERS.
Cost sharing for a month's supply of insulin is capped at $35, and for a covered inhaler at $35, for enrollees in Medicare and in group and individual market plans.

SEC. 5. OUT-OF-POCKET CAP.
The annual out-of-pocket threshold under Medicare part D is reduced from $2,000 to $1,500, indexed thereafter to the consumer price index rather than to per capita part D spending.

SEC. 6. PHARMACY BENEFIT MANAGER TRANSPARENCY.
(a) Spread pricing is prohibited in Medicaid managed care and Medicare part D.
(b) A pharmacy benefit manager shall pass through 100 percent of rebates to the plan sponsor and shall report aggregate rebate, fee, and net price data to the Secretary semiannually.

SEC. 7. GENERIC AND BIOSIMILAR COMPETITION.
Product hopping and the abuse of citizen petitions to delay generic entry are designated unfair methods of competition. Pay-for-delay settlements are presumptively anticompetitive.

SEC. 8. SAVINGS.
Amounts saved under this Act shall be credited to the Federal Supplementary Medical Insurance Trust Fund.`,
			PolicyArea:     "Health",
			Sponsor:        "Sen. Bennet, Michael F. [D-CO]",
			SponsorParty:   "D",
			IdeologyScore:  -0.58,
			IntroducedDate: day(2025, time.March, 13),
			UpdatedAt:      day(2025, time.May, 6),
		},
		{
			ID:      "hr-2890",
			Number:  "H.R. 2890",
			Title:   "AI Research and Innovation Investment Act",
			Chamber: models.ChamberHouse,
			Status:  "Referred to the Committee on Science, Space, and Technology",
			Summary: "Invests in AI research, workforce development, and public-private partnerships to maintain U.S. leadership in AI.",
			FullText: `SECTION 1. SHORT TITLE.
This Act may be cited as the "AI Research and Innovation Investment Act".

SEC. 2. NATIONAL AI RESEARCH RESOURCE.
There is authorized to be appropriated $2,600,000,000 over 6 years to establish a shared national research infrastructure providing compute, curated datasets, and testbeds to academic and non-profit researchers, administered by the National Science Foundation.

SEC. 3. STANDARDS AND EVALUATION.
The National Institute of Standards and Technology shall develop voluntary consensus standards for the evaluation of frontier model capabilities, including benchmarks for reliability, security, and misuse potential, and shall operate a testing program open to developers.

SEC. 4. WORKFORCE.
(a) $900,000,000 is authorized for AI traineeships, community college certificate programs, and teacher professional development.
(b) A scholarship-for-service program is established for graduates who serve in Federal AI roles.

SEC. 5. PUBLIC-PRIVATE PARTNERSHIPS.
The Secretary of Commerce shall establish not fewer than 8 regional AI innovation institutes pairing universities with private partners, with a 30 percent non-Federal cost share.

SEC. 6. APPLIED RESEARCH PRIORITIES.
Priority is given to applications in materials discovery, grid optimization, drug development, agricultural productivity, and government service delivery.

SEC. 7. SECURITY.
Recipients shall implement research security plans and disclose foreign talent program participation. Nothing in this Act authorizes a new licensing regime for the development or release of models.

SEC. 8. OVERSIGHT.
The Comptroller General shall report to Congress every 2 years on the return on Federal AI research investment.`,
			PolicyArea:     "Science, Technology, Communications",
			Sponsor:        "Rep. Obernolte, Jay [R-CA]",
			SponsorParty:   "R",
			IdeologyScore:  -0.18,
			IntroducedDate: day(2025, time.April, 15),
			UpdatedAt:      day(2025, time.May, 5),
		},
		{
			ID:      "s-875",
			Number:  "S. 875",
			Title:   "National Cybersecurity Resilience Act",
			Chamber: models.ChamberSenate,
			Status:  "Reported by the Committee on Homeland Security and Governmental Affairs",
			Summary: "Strengthens critical infrastructure protections and enhances federal coordination on cybersecurity threats.",
			FullText: `SECTION 1. SHORT TITLE.
This Act may be cited as the "National Cybersecurity Resilience Act".

SEC. 2. INCIDENT REPORTING.
An owner or operator of covered critical infrastructure shall report a substantial cyber incident to the Cybersecurity and Infrastructure Security Agency within 72 hours, and a ransom payment within 24 hours. Reports are protected from disclosure and may not be used as direct evidence in an enforcement action.

SEC. 3. MINIMUM PRACTICES.
Sector risk management agencies shall establish minimum cybersecurity practices for covered entities, including multifactor authentication, network segmentation, tested offline backups, and a documented recovery time objective.

SEC. 4. FEDERAL NETWORKS.
(a) Federal civilian agencies shall complete zero trust architecture milestones within 3 years.
(b) The Federal Chief Information Security Officer shall maintain a continuous inventory of internet-facing assets across the civilian enterprise.

SEC. 5. STATE, LOCAL, AND RURAL SUPPORT.
$1,200,000,000 is authorized for the State and Local Cybersecurity Grant Program, with a set-aside for rural water systems, rural hospitals, and school districts.

SEC. 6. WORKFORCE.
A Cyber Service Academy scholarship program is expanded, and agencies are granted direct hire authority for cybersecurity positions through 2032.

SEC. 7. INTERNATIONAL AND SUPPLY CHAIN.
The Secretary shall maintain a list of hardware and software vendors that present an unacceptable supply chain risk, and shall coordinate joint attribution and sanctions recommendations with allied governments.

SEC. 8. SUNSET.
The reporting requirements under section 2 expire 8 years after the date of enactment unless reauthorized.`,
			PolicyArea:     "Science, Technology, Communications",
			Sponsor:        "Sen. Peters, Gary C. [D-MI]",
			SponsorParty:   "D",
			IdeologyScore:  0.08,
			IntroducedDate: day(2025, time.March, 5),
			UpdatedAt:      day(2025, time.May, 5),
		},
		{
			ID:      "hr-1567",
			Number:  "H.R. 1567",
			Title:   "Affordable Housing Supply Act",
			Chamber: models.ChamberHouse,
			Status:  "Referred to the Committee on Financial Services",
			Summary: "Incentivizes the construction of affordable housing and streamlines local permitting processes.",
			FullText: `SECTION 1. SHORT TITLE.
This Act may be cited as the "Affordable Housing Supply Act".

SEC. 2. LOW-INCOME HOUSING TAX CREDIT.
(a) The State housing credit ceiling is increased by 50 percent and indexed for inflation.
(b) The private activity bond financing threshold for the 4 percent credit is reduced from 50 percent to 25 percent.

SEC. 3. NEIGHBORHOOD HOMES CREDIT.
A new credit is established for the construction or substantial rehabilitation of owner-occupied homes in distressed census tracts, limited to the gap between development cost and appraised value.

SEC. 4. UNLOCKING LOCAL SUPPLY.
(a) A jurisdiction receiving Community Development Block Grant funds shall submit a housing supply plan addressing minimum lot sizes, parking mandates, and multifamily zoning near transit.
(b) $3,000,000,000 is authorized for competitive grants to jurisdictions that adopt by-right approval for qualifying projects and reduce median permitting time below 90 days.

SEC. 5. HOUSING TRUST FUND.
The Housing Trust Fund is capitalized with an additional $4,000,000,000 for extremely low-income rental construction.

SEC. 6. VOUCHERS AND STABILITY.
An additional 250,000 housing choice vouchers are authorized, with landlord incentive payments and a Federal right to counsel pilot in eviction proceedings in 20 jurisdictions.

SEC. 7. MANUFACTURED AND MODULAR HOUSING.
The Secretary shall update the manufactured housing construction standards to permit multi-unit and duplex configurations, and shall expand title I loan limits.

SEC. 8. FEDERAL LAND.
Surplus Federal land may be conveyed at a discount to a public housing agency or non-profit developer for projects in which at least 50 percent of units are affordable at 60 percent of area median income.`,
			PolicyArea:     "Housing and Community Development",
			Sponsor:        "Rep. Waters, Maxine [D-CA]",
			SponsorParty:   "D",
			IdeologyScore:  -0.45,
			IntroducedDate: day(2025, time.February, 25),
			UpdatedAt:      day(2025, time.May, 4),
		},
		{
			ID:      "s-742",
			Number:  "S. 742",
			Title:   "Small Business Tax Relief and Simplification Act",
			Chamber: models.ChamberSenate,
			Status:  "Read twice and referred to the Committee on Finance",
			Summary: "Makes the qualified business income deduction permanent, restores immediate expensing, and simplifies filing for small employers.",
			FullText: `SECTION 1. SHORT TITLE.
This Act may be cited as the "Small Business Tax Relief and Simplification Act".

SEC. 2. QUALIFIED BUSINESS INCOME.
The 20 percent deduction for qualified business income under section 199A of the Internal Revenue Code is made permanent and the phase-in thresholds are increased to $500,000 for individual filers.

SEC. 3. EXPENSING.
(a) Full expensing of qualified property under section 168(k) is restored and made permanent.
(b) Domestic research and experimental expenditures may again be deducted in the year incurred rather than amortized over 5 years.

SEC. 4. SIMPLIFICATION.
(a) The gross receipts threshold for use of the cash method of accounting is raised to $50,000,000.
(b) The Secretary shall publish a single simplified return for businesses with fewer than 25 employees and shall not require paper filing for any small employer form.

SEC. 5. STARTUP COSTS.
The deduction for startup and organizational expenditures is increased from $5,000 to $50,000.

SEC. 6. COMPLIANCE RELIEF.
Beneficial ownership reporting deadlines under the Corporate Transparency Act are extended, and a first-time penalty waiver is established for good faith errors by employers with fewer than 50 employees.

SEC. 7. OFFSET.
The Joint Committee on Taxation shall report the revenue effect of this Act; amounts not offset shall be subject to the pay-as-you-go scorecard.`,
			PolicyArea:     "Taxation",
			Sponsor:        "Sen. Ernst, Joni [R-IA]",
			SponsorParty:   "R",
			IdeologyScore:  0.7,
			IntroducedDate: day(2025, time.February, 26),
			UpdatedAt:      day(2025, time.May, 2),
		},
		{
			ID:      "hr-4102",
			Number:  "H.R. 4102",
			Title:   "Farmland Preservation and Rural Investment Act",
			Chamber: models.ChamberHouse,
			Status:  "Referred to the Committee on Agriculture",
			Summary: "Protects working farmland from conversion, expands conservation easements, and funds rural broadband and processing capacity.",
			FullText: `SECTION 1. SHORT TITLE.
This Act may be cited as the "Farmland Preservation and Rural Investment Act".

SEC. 2. AGRICULTURAL LAND EASEMENTS.
Funding for the Agricultural Conservation Easement Program is increased to $1,000,000,000 annually, with priority for farms in counties experiencing the highest rate of conversion to non-agricultural use.

SEC. 3. BEGINNING FARMERS.
(a) A down payment loan program is expanded with a reduced interest rate for beginning, veteran, and socially disadvantaged producers.
(b) A tax credit is provided to landowners who sell or lease farmland to a beginning farmer.

SEC. 4. FOREIGN OWNERSHIP.
Acquisition of agricultural land by an entity controlled by a country of concern is prohibited, and reporting under the Agricultural Foreign Investment Disclosure Act is modernized with civil penalties for nondisclosure.

SEC. 5. PROCESSING CAPACITY.
$600,000,000 is authorized for grants and guaranteed loans to small and mid-sized meat and poultry processors, dairy processors, and grain handling facilities.

SEC. 6. RURAL BROADBAND.
The ReConnect Program is reauthorized at $2,000,000,000 with a minimum service standard of 100/20 megabits per second and a preference for open-access middle mile.

SEC. 7. CONSERVATION PRACTICES.
Cost-share rates are increased for cover crops, precision nutrient management, and irrigation efficiency, and a voluntary soil carbon measurement pilot is established.

SEC. 8. REPORT.
The Secretary shall report annually on acres of farmland converted, easements enrolled, and processing capacity added.`,
			PolicyArea:     "Agriculture and Food",
			Sponsor:        "Rep. Craig, Angie [D-MN]",
			SponsorParty:   "D",
			IdeologyScore:  -0.1,
			IntroducedDate: day(2025, time.April, 8),
			UpdatedAt:      day(2025, time.May, 1),
		},
		{
			ID:      "s-690",
			Number:  "S. 690",
			Title:   "Election Integrity and Voter Access Act",
			Chamber: models.ChamberSenate,
			Status:  "Read twice and referred to the Committee on Rules and Administration",
			Summary: "Pairs new voter list maintenance and audit requirements with minimum standards for early voting and mail ballot cure.",
			FullText: `SECTION 1. SHORT TITLE.
This Act may be cited as the "Election Integrity and Voter Access Act".

SEC. 2. LIST MAINTENANCE.
States shall conduct annual list maintenance using death records, State motor vehicle records, and interstate cross-checks, and shall publish the number of registrations removed by category. A registrant may not be removed solely for failure to vote.

SEC. 3. AUDITS.
Each State shall conduct a risk-limiting audit of Federal contests before certification, using a voter-verifiable paper record for every ballot cast.

SEC. 4. EQUIPMENT SECURITY.
Voting systems shall be certified under updated Voluntary Voting System Guidelines, may not be connected to the internet, and shall be subject to chain-of-custody logging and post-election logic and accuracy testing.

SEC. 5. MINIMUM ACCESS STANDARDS.
(a) Each State shall provide not fewer than 10 days of early in-person voting, including one weekend.
(b) A voter whose mail ballot is rejected for a signature or technical defect shall receive notice and an opportunity to cure not later than 5 days after the election.
(c) Online voter registration shall be available in every State.

SEC. 6. IDENTIFICATION.
A State that requires photo identification shall provide such identification at no cost and shall accept a signed affidavit and a provisional ballot from a voter who lacks it.

SEC. 7. ELECTION ADMINISTRATION FUNDING.
$800,000,000 is authorized for the Election Assistance Commission for equipment replacement, cybersecurity, and poll worker recruitment.

SEC. 8. PROTECTION OF ELECTION OFFICIALS.
Threatening an election official or interfering with the transmission of election results is subject to enhanced Federal penalties.`,
			PolicyArea:     "Government Operations and Politics",
			Sponsor:        "Sen. Collins, Susan M. [R-ME]",
			SponsorParty:   "R",
			IdeologyScore:  0.3,
			IntroducedDate: day(2025, time.February, 20),
			UpdatedAt:      day(2025, time.April, 30),
		},
		{
			ID:      "hr-3355",
			Number:  "H.R. 3355",
			Title:   "Clean Water Infrastructure Modernization Act",
			Chamber: models.ChamberHouse,
			Status:  "Passed House, referred to the Committee on Environment and Public Works",
			Summary: "Recapitalizes state revolving funds, accelerates lead service line replacement, and sets deadlines for PFAS remediation.",
			FullText: `SECTION 1. SHORT TITLE.
This Act may be cited as the "Clean Water Infrastructure Modernization Act".

SEC. 2. STATE REVOLVING FUNDS.
The Clean Water and Drinking Water State Revolving Funds are reauthorized at a combined $9,000,000,000 annually through fiscal year 2031, with not less than 25 percent provided as principal forgiveness to disadvantaged communities.

SEC. 3. LEAD SERVICE LINES.
(a) Water systems shall complete a full inventory of service line materials within 2 years and replace all lead service lines within 10 years.
(b) $5,000,000,000 is authorized for replacement, and a system may not charge a customer for the private-side portion of a replacement funded under this section.

SEC. 4. PFAS.
The Administrator shall establish enforceable limits for additional perfluoroalkyl and polyfluoroalkyl substances within 3 years, and shall provide treatment grants to small and rural systems. Passive receivers of PFAS, including public water utilities and airports, are shielded from liability under CERCLA.

SEC. 5. STORMWATER AND RESILIENCE.
A competitive grant program is established for green infrastructure, combined sewer overflow abatement, and flood resilience at wastewater treatment plants.

SEC. 6. WORKFORCE.
$100,000,000 is authorized for water operator apprenticeships and certification, addressing the projected retirement of one-third of the operator workforce.

SEC. 7. AFFORDABILITY.
The Low Income Household Water Assistance Program is made permanent, and systems receiving assistance shall report on shutoff practices.

SEC. 8. BUY AMERICA.
Iron, steel, and manufactured products used in projects assisted under this Act shall be produced in the United States, subject to existing waiver authority.`,
			PolicyArea:     "Water Resources Development",
			Sponsor:        "Rep. Napolitano, Grace F. [D-CA]",
			SponsorParty:   "D",
			IdeologyScore:  -0.35,
			IntroducedDate: day(2025, time.January, 30),
			UpdatedAt:      day(2025, time.April, 29),
		},
	}

	for i := range bills {
		bills[i].Source = models.SourceSeed
		bills[i].StatusCategory = models.StatusCategory(bills[i].Status)
	}
	return bills
}
