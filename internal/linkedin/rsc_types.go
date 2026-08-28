package linkedin

type InternalProfile struct {
	Slug             string
	PublicIdentifier string
	VieweeProfileID  string
	FirstName        string
	LastName         string
	FullName         string
	Headline         string
	About            string
	City             string
	Region           string
	Country          string
	ProfileImageURL  string
	Experience       []RawExperienceItem
	Education        []RawEducationItem
	Certifications   []RawCertificationItem
	Languages        []RawLanguageItem
	Skills           []RawSkillItem
	Featured         []RawFeaturedItem
}

type RawProfileTopCard struct {
	VieweeProfileID string
	FirstName       string
	LastName        string
	FullName        string
	Headline        string
	City            string
	Region          string
	Country         string
	ProfileImageURL string
}

type RawExperienceItem struct {
	Title       string
	Company     string
	CompanyURL  string
	Location    string
	Description string
	StartMonth  int
	StartYear   int
	EndMonth    int
	EndYear     int
	Current     bool
}

type RawEducationItem struct {
	School       string
	Degree       string
	FieldOfStudy string
	StartYear    int
	EndYear      int
}

type RawCertificationItem struct {
	Name      string
	Authority string
	URL       string
}

type RawLanguageItem struct {
	Name        string
	Proficiency string
}

type RawSkillItem struct {
	Name string
}

type RawFeaturedItem struct {
	Type        string
	Title       string
	Description string
	URL         string
}
