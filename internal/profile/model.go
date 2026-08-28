package profile

type Profile struct {
	ProfileURL       string          `json:"profile_url"`
	PublicIdentifier string          `json:"public_identifier"`
	Name             Name            `json:"name"`
	Headline         string          `json:"headline,omitempty"`
	About            string          `json:"about,omitempty"`
	Location         *Location       `json:"location,omitempty"`
	ProfileImage     *Image          `json:"profile_image,omitempty"`
	Experience       []Experience    `json:"experience"`
	Education        []Education     `json:"education"`
	Skills           []Skill         `json:"skills"`
	Certifications   []Certification `json:"certifications"`
	Languages        []Language      `json:"languages"`
}

type Name struct {
	First string `json:"first"`
	Last  string `json:"last"`
	Full  string `json:"full"`
}

type Location struct {
	City    string `json:"city,omitempty"`
	Region  string `json:"region,omitempty"`
	Country string `json:"country,omitempty"`
}

type Image struct {
	URL string `json:"url"`
}

type Experience struct {
	Title              string `json:"title,omitempty"`
	Company            string `json:"company,omitempty"`
	CompanyLinkedInURL string `json:"company_linkedin_url,omitempty"`
	Location           string `json:"location,omitempty"`
	Description        string `json:"description,omitempty"`
	StartDate          *Date  `json:"start_date,omitempty"`
	EndDate            *Date  `json:"end_date"`
	Current            bool   `json:"current"`
}

type Education struct {
	School       string `json:"school,omitempty"`
	Degree       string `json:"degree,omitempty"`
	FieldOfStudy string `json:"field_of_study,omitempty"`
	Description  string `json:"description,omitempty"`
	StartDate    *Date  `json:"start_date,omitempty"`
	EndDate      *Date  `json:"end_date,omitempty"`
}

type Date struct {
	Month int `json:"month,omitempty"`
	Year  int `json:"year,omitempty"`
}

type Skill struct {
	Name string `json:"name"`
}

type Certification struct {
	Name          string `json:"name,omitempty"`
	Authority     string `json:"authority,omitempty"`
	LicenseNumber string `json:"license_number,omitempty"`
	URL           string `json:"url,omitempty"`
	StartDate     *Date  `json:"start_date,omitempty"`
	EndDate       *Date  `json:"end_date,omitempty"`
}

type Language struct {
	Name        string `json:"name"`
	Proficiency string `json:"proficiency,omitempty"`
}
