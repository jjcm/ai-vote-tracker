package seed

import (
	"time"

	"github.com/pwnies/ai-vote-tracker/internal/models"
)

// The appropriations bill is the long one in the corpus. Its account-level
// sections grouped under titles and divisions are what an appropriations act
// actually looks like, and they give the offline corpus a bill that reads the
// way the largest live bills do: many short provisions rather than a few long
// ones.
func appropriations() models.Bill {
	const name = "Consolidated Appropriations Act, 2026"
	return models.Bill{
		ID:      "hr-4890",
		Number:  "H.R. 4890",
		Title:   name,
		Chamber: models.ChamberHouse,
		Status:  "Reported by the Committee on Appropriations",
		Summary: "Provides full-year appropriations for the Departments of the Interior, Transportation, Commerce, Energy, and related agencies for fiscal year 2026.",
		FullText: appropriationsXML("H.R. 4890", name, models.ChamberHouse,
			"Making appropriations for the Departments of the Interior, Transportation, Commerce, Energy, and related agencies for the fiscal year ending September 30, 2026, and for other purposes.",
			[]division{
				{
					Enum:   "A",
					Header: "Interior, Environment, and Related Agencies",
					Titles: []billTitle{
						{
							Enum:   "I",
							Header: "Department of the Interior",
							Accounts: []account{
								{
									Agency:  "the Bureau of Land Management",
									Header:  "Management of Lands and Resources",
									Amount:  "$1,412,000,000",
									Purpose: "the management of public lands, including grazing administration, recreation, and cadastral survey",
									Proviso: []string{
										"not less than $60,000,000 shall be available for the sage-grouse and mule deer habitat restoration initiative;",
										"no funds may be used to close a public access route without prior notice in the Federal Register.",
									},
								},
								{
									Agency:  "the United States Fish and Wildlife Service",
									Header:  "Resource Management",
									Amount:  "$1,684,000,000",
									Purpose: "ecological services, the National Wildlife Refuge System, and international affairs",
									Proviso: []string{
										"not less than $34,000,000 shall be for candidate conservation agreements with private landowners.",
									},
								},
								{
									Agency:  "the National Park Service",
									Header:  "Operation of the National Park System",
									Amount:  "$3,061,000,000",
									Purpose: "the operation, maintenance, and interpretation of the National Park System",
									Proviso: []string{
										"not less than $30,000,000 shall be for deferred maintenance at units with the highest visitation growth;",
										"amounts collected under recreation fee authority shall remain available without further appropriation.",
									},
								},
								{
									Agency:  "the Bureau of Indian Affairs",
									Header:  "Operation of Indian Programs",
									Amount:  "$2,110,000,000",
									Purpose: "Tribal government services, public safety and justice, and human services",
									Proviso: []string{
										"contract support costs shall be paid in full as an indefinite appropriation.",
									},
								},
							},
						},
						{
							Enum:   "II",
							Header: "Environmental Protection Agency",
							Accounts: []account{
								{
									Agency:  "the Environmental Protection Agency",
									Header:  "Environmental Programs and Management",
									Amount:  "$3,247,000,000",
									Purpose: "environmental programs and management, including geographic programs and enforcement support",
									Proviso: []string{
										"not less than $400,000,000 shall be for the Great Lakes Restoration Initiative;",
										"no funds may be used to implement a rule that has been stayed by a court of competent jurisdiction.",
									},
								},
								{
									Agency:  "the Environmental Protection Agency",
									Header:  "State and Tribal Assistance Grants",
									Amount:  "$4,912,000,000",
									Purpose: "capitalization grants for the Clean Water and Drinking Water State Revolving Funds and categorical grants to States and Tribes",
									Proviso: []string{
										"not less than 20 percent of each capitalization grant shall be used for principal forgiveness for disadvantaged communities;",
										"not less than $70,000,000 shall be for grants to address emerging contaminants in small and disadvantaged communities.",
									},
								},
								{
									Agency:  "the Environmental Protection Agency",
									Header:  "Hazardous Substance Superfund",
									Amount:  "$1,306,000,000",
									Purpose: "remedial action, removal actions, and enforcement at sites on the National Priorities List",
									Proviso: []string{
										"not less than $30,000,000 shall be for the emergency response and removal program.",
									},
								},
							},
						},
						{
							Enum:   "III",
							Header: "Related Agencies",
							Accounts: []account{
								{
									Agency:  "the Forest Service",
									Header:  "Wildland Fire Management",
									Amount:  "$2,458,000,000",
									Purpose: "wildland fire preparedness, suppression operations, and hazardous fuels management",
									Proviso: []string{
										"not less than $760,000,000 shall be for hazardous fuels reduction in the wildland-urban interface;",
										"the Secretary may transfer amounts between preparedness and suppression upon notification of the Committees on Appropriations.",
									},
								},
								{
									Agency:  "the Forest Service",
									Header:  "National Forest System",
									Amount:  "$1,996,000,000",
									Purpose: "the management of the National Forest System, including timber sales, grazing, and recreation",
									Proviso: []string{
										"not less than $50,000,000 shall be for the reforestation of areas burned in the preceding 5 fiscal years.",
									},
								},
								{
									Agency:  "the Smithsonian Institution",
									Header:  "Salaries and Expenses",
									Amount:  "$924,000,000",
									Purpose: "the operation and maintenance of museums, research centers, and the National Zoological Park",
								},
							},
						},
					},
				},
				{
					Enum:   "B",
					Header: "Transportation, Housing and Urban Development, and Related Agencies",
					Titles: []billTitle{
						{
							Enum:   "I",
							Header: "Department of Transportation",
							Accounts: []account{
								{
									Agency:  "the Federal Aviation Administration",
									Header:  "Operations",
									Amount:  "$13,420,000,000",
									Purpose: "air traffic services, aviation safety inspection, and commercial space transportation oversight",
									Proviso: []string{
										"not less than $200,000,000 shall be for the hiring and training of air traffic controllers to reach the staffing standard;",
										"no funds may be used to reduce the number of certificated towers below the level in effect on the date of enactment.",
									},
								},
								{
									Agency:  "the Federal Highway Administration",
									Header:  "Federal-Aid Highways",
									Amount:  "$62,800,000,000",
									Purpose: "the obligation limitation on Federal-aid highway and highway safety construction programs",
									Proviso: []string{
										"not less than $1,400,000,000 shall be for the bridge formula program;",
										"amounts shall be distributed in the ratio prescribed by section 104 of title 23, United States Code.",
									},
								},
								{
									Agency:  "the Federal Transit Administration",
									Header:  "Capital Investment Grants",
									Amount:  "$3,190,000,000",
									Purpose: "new fixed guideway capital projects, core capacity improvements, and small starts projects",
									Proviso: []string{
										"not less than $500,000,000 shall be for projects in urbanized areas with populations of less than 200,000.",
									},
								},
								{
									Agency:  "the Federal Railroad Administration",
									Header:  "Northeast Corridor Grants to Amtrak",
									Amount:  "$1,260,000,000",
									Purpose: "operating and capital grants for the Northeast Corridor, including state-of-good-repair backlog reduction",
								},
							},
						},
						{
							Enum:   "II",
							Header: "Department of Housing and Urban Development",
							Accounts: []account{
								{
									Agency:  "the Department of Housing and Urban Development",
									Header:  "Tenant-Based Rental Assistance",
									Amount:  "$32,700,000,000",
									Purpose: "the renewal of housing choice vouchers, administrative fees, and tenant protection vouchers",
									Proviso: []string{
										"not less than $500,000,000 shall be for incremental vouchers for households experiencing homelessness;",
										"no public housing agency may reduce the number of vouchers under lease below the level funded by this Act.",
									},
								},
								{
									Agency:  "the Department of Housing and Urban Development",
									Header:  "Community Development Block Grants",
									Amount:  "$3,300,000,000",
									Purpose: "formula grants to entitlement communities and States for community development activities",
									Proviso: []string{
										"a grantee shall submit the housing supply plan required by section 5 of the Affordable Housing Supply Act if that Act is enacted.",
									},
								},
								{
									Agency:  "the Department of Housing and Urban Development",
									Header:  "Homeless Assistance Grants",
									Amount:  "$4,050,000,000",
									Purpose: "continuum of care grants, emergency solutions grants, and rural set-aside programs",
									Proviso: []string{
										"not less than $290,000,000 shall be for youth homelessness demonstration projects.",
									},
								},
							},
						},
					},
				},
				{
					Enum:   "C",
					Header: "Energy and Water Development, and Related Agencies",
					Titles: []billTitle{
						{
							Enum:   "I",
							Header: "Department of Energy",
							Accounts: []account{
								{
									Agency:  "the Office of Energy Efficiency and Renewable Energy",
									Header:  "Energy Efficiency and Renewable Energy",
									Amount:  "$3,460,000,000",
									Purpose: "research, development, and demonstration in solar, wind, geothermal, hydrogen, and industrial efficiency",
									Proviso: []string{
										"not less than $120,000,000 shall be for the weatherization assistance program.",
									},
								},
								{
									Agency:  "the Office of Nuclear Energy",
									Header:  "Nuclear Energy",
									Amount:  "$1,780,000,000",
									Purpose: "advanced reactor demonstration, fuel cycle research, and the domestic high-assay low-enriched uranium supply",
									Proviso: []string{
										"not less than $300,000,000 shall be for the advanced reactor demonstration program.",
									},
								},
								{
									Agency:  "the Office of Science",
									Header:  "Science",
									Amount:  "$8,900,000,000",
									Purpose: "basic research in high energy physics, fusion energy sciences, advanced scientific computing, and biological and environmental research",
									Proviso: []string{
										"not less than $1,000,000,000 shall be for exascale and post-exascale computing facilities.",
									},
								},
								{
									Agency:  "the Advanced Research Projects Agency—Energy",
									Header:  "Advanced Research Projects Agency—Energy",
									Amount:  "$470,000,000",
									Purpose: "high-potential, high-impact energy technologies that are too early for private sector investment",
								},
							},
						},
						{
							Enum:   "II",
							Header: "Corps of Engineers—Civil",
							Accounts: []account{
								{
									Agency:  "the Corps of Engineers",
									Header:  "Construction",
									Amount:  "$2,900,000,000",
									Purpose: "the construction of river and harbor, flood and storm damage reduction, and aquatic ecosystem restoration projects",
									Proviso: []string{
										"not less than $200,000,000 shall be for projects in rural communities with a population of less than 25,000.",
									},
								},
								{
									Agency:  "the Corps of Engineers",
									Header:  "Operation and Maintenance",
									Amount:  "$5,410,000,000",
									Purpose: "the operation, maintenance, and dredging of authorized navigation, flood control, and hydropower projects",
									Proviso: []string{
										"amounts from the Harbor Maintenance Trust Fund shall be expended in accordance with section 14003 of the Water Resources Development Act of 2020.",
									},
								},
							},
						},
					},
				},
				{
					Enum:   "D",
					Header: "Commerce, Justice, Science, and Related Agencies",
					Titles: []billTitle{
						{
							Enum:   "I",
							Header: "Department of Commerce",
							Accounts: []account{
								{
									Agency:  "the National Oceanic and Atmospheric Administration",
									Header:  "Operations, Research, and Facilities",
									Amount:  "$4,320,000,000",
									Purpose: "the National Weather Service, fisheries management, ocean and coastal management, and climate research",
									Proviso: []string{
										"not less than $1,300,000,000 shall be for the National Weather Service, of which not less than $30,000,000 shall be for the modernization of flood forecasting.",
									},
								},
								{
									Agency:  "the National Institute of Standards and Technology",
									Header:  "Scientific and Technical Research and Services",
									Amount:  "$1,010,000,000",
									Purpose: "measurement science, standards development, and the artificial intelligence evaluation program",
									Proviso: []string{
										"not less than $60,000,000 shall be for the evaluation of frontier model capabilities.",
									},
								},
								{
									Agency:  "the Census Bureau",
									Header:  "Periodic Censuses and Programs",
									Amount:  "$1,470,000,000",
									Purpose: "the decennial census, the American Community Survey, and economic census programs",
								},
							},
						},
						{
							Enum:   "II",
							Header: "Department of Justice",
							Accounts: []account{
								{
									Agency:  "the Federal Bureau of Investigation",
									Header:  "Salaries and Expenses",
									Amount:  "$11,340,000,000",
									Purpose: "criminal investigation, counterterrorism, counterintelligence, and cyber investigative activities",
									Proviso: []string{
										"not less than $300,000,000 shall be for cyber investigative capacity and victim notification.",
									},
								},
								{
									Agency:  "the Office of Justice Programs",
									Header:  "State and Local Law Enforcement Assistance",
									Amount:  "$2,180,000,000",
									Purpose: "Byrne justice assistance grants, victims of crime programs, and rural violent crime initiatives",
									Proviso: []string{
										"not less than $100,000,000 shall be for the rural violent crime initiative.",
									},
								},
								{
									Agency:  "the Drug Enforcement Administration",
									Header:  "Salaries and Expenses",
									Amount:  "$2,760,000,000",
									Purpose: "the enforcement of the Controlled Substances Act and the disruption of fentanyl trafficking networks",
								},
							},
						},
						{
							Enum:   "III",
							Header: "Science",
							Accounts: []account{
								{
									Agency:  "the National Science Foundation",
									Header:  "Research and Related Activities",
									Amount:  "$7,890,000,000",
									Purpose: "research and education in science and engineering, including the national artificial intelligence research resource",
									Proviso: []string{
										"not less than $400,000,000 shall be for the national artificial intelligence research resource;",
										"not less than 25 percent of amounts for that resource shall be allocated to institutions that are not among the 25 largest recipients of Federal research funding.",
									},
								},
								{
									Agency:  "the National Aeronautics and Space Administration",
									Header:  "Science",
									Amount:  "$7,620,000,000",
									Purpose: "earth science, planetary science, astrophysics, and heliophysics missions",
									Proviso: []string{
										"not less than $300,000,000 shall be for the Mars sample return mission.",
									},
								},
							},
						},
					},
				},
				{
					Enum:   "E",
					Header: "General Provisions—This Act",
					Titles: []billTitle{
						{
							Enum:   "I",
							Header: "General Provisions",
							Accounts: []account{
								{
									Agency:  "each agency funded by this Act",
									Header:  "Reprogramming limitation",
									Amount:  "no additional amount is appropriated",
									Purpose: "the reprogramming of funds",
									Proviso: []string{
										"no amount may be reprogrammed between programs, projects, or activities in excess of $5,000,000 or 10 percent, whichever is less, without prior notification of the Committees on Appropriations;",
										"a notification under this section shall include the reason for the change and its effect on the affected activity.",
									},
								},
								{
									Agency:  "each agency funded by this Act",
									Header:  "Availability of funds",
									Amount:  "no additional amount is appropriated",
									Purpose: "the availability of amounts appropriated by this Act",
									Proviso: []string{
										"amounts provided by this Act shall not be available for obligation after September 30, 2028, unless expressly provided otherwise;",
										"no amount may be used to pay the salary of an officer whose nomination requires the advice and consent of the Senate and who is serving in an acting capacity beyond 300 days.",
									},
								},
								{
									Agency:  "each agency funded by this Act",
									Header:  "Reporting",
									Amount:  "no additional amount is appropriated",
									Purpose: "the reporting requirements of this Act",
									Proviso: []string{
										"each agency shall submit a spend plan to the Committees on Appropriations within 60 days after the date of enactment;",
										"each agency shall report quarterly on unobligated balances by program, project, and activity.",
									},
								},
							},
						},
					},
				},
			}),
		PolicyArea:     "Economics and Public Finance",
		Sponsor:        "Rep. Cole, Tom [R-OK]",
		SponsorParty:   "R",
		IdeologyScore:  0.12,
		IntroducedDate: day(2025, time.April, 22),
		UpdatedAt:      day(2025, time.April, 28),
	}
}
