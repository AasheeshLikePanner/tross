package profile

import (
	"fmt"
	"sort"
	"strings"

	"github.com/tross/linkedin-profile-api/internal/linkedin"
)

func Normalize(publicIdentifier, profileURL string, raw *linkedin.InternalProfile) (*Profile, error) {
	if raw == nil {
		return nil, fmt.Errorf("normalize: nil internal profile")
	}

	p := &Profile{
		ProfileURL:       profileURL,
		PublicIdentifier: publicIdentifier,
		Name: Name{
			First: raw.FirstName,
			Last:  raw.LastName,
			Full:  strings.TrimSpace(raw.FullName),
		},
		Headline: raw.Headline,
		About:    raw.About,
	}

	if p.Name.Full == "" && (raw.FirstName != "" || raw.LastName != "") {
		p.Name.Full = strings.TrimSpace(raw.FirstName + " " + raw.LastName)
	}

	if raw.City != "" || raw.Region != "" || raw.Country != "" {
		p.Location = &Location{
			City:    raw.City,
			Region:  raw.Region,
			Country: raw.Country,
		}
	}

	if raw.ProfileImageURL != "" {
		p.ProfileImage = &Image{URL: raw.ProfileImageURL}
	}

	// Experience
	p.Experience = make([]Experience, 0, len(raw.Experience))
	for _, exp := range raw.Experience {
		e := Experience{
			Title:              exp.Title,
			Company:            exp.Company,
			CompanyLinkedInURL: exp.CompanyURL,
			Location:           exp.Location,
			Description:        exp.Description,
			Current:            exp.Current,
		}
		if exp.StartYear > 0 {
			e.StartDate = &Date{Month: exp.StartMonth, Year: exp.StartYear}
		}
		if exp.EndYear > 0 && !exp.Current {
			e.EndDate = &Date{Month: exp.EndMonth, Year: exp.EndYear}
		}
		p.Experience = append(p.Experience, e)
	}

	// Sort experience: current roles first, then newest start date
	sort.SliceStable(p.Experience, func(i, j int) bool {
		ei, ej := p.Experience[i], p.Experience[j]
		if ei.Current != ej.Current {
			return ei.Current
		}
		yi, yj := 0, 0
		if ei.StartDate != nil {
			yi = ei.StartDate.Year*100 + ei.StartDate.Month
		}
		if ej.StartDate != nil {
			yj = ej.StartDate.Year*100 + ej.StartDate.Month
		}
		return yi > yj
	})

	// Education
	p.Education = make([]Education, 0, len(raw.Education))
	for _, edu := range raw.Education {
		ed := Education{
			School:       edu.School,
			Degree:       edu.Degree,
			FieldOfStudy: edu.FieldOfStudy,
		}
		if edu.StartYear > 0 {
			ed.StartDate = &Date{Year: edu.StartYear}
		}
		if edu.EndYear > 0 {
			ed.EndDate = &Date{Year: edu.EndYear}
		}
		p.Education = append(p.Education, ed)
	}

	// Skills
	seenSkills := make(map[string]bool)
	p.Skills = make([]Skill, 0, len(raw.Skills))
	for _, s := range raw.Skills {
		name := strings.TrimSpace(s.Name)
		if name == "" || seenSkills[strings.ToLower(name)] {
			continue
		}
		seenSkills[strings.ToLower(name)] = true
		p.Skills = append(p.Skills, Skill{Name: name})
	}

	// Certifications
	p.Certifications = make([]Certification, 0, len(raw.Certifications))
	for _, cert := range raw.Certifications {
		if cert.Name == "" {
			continue
		}
		c := Certification{
			Name:      cert.Name,
			Authority: cert.Authority,
			URL:       cert.URL,
		}
		p.Certifications = append(p.Certifications, c)
	}

	// Languages
	p.Languages = make([]Language, 0, len(raw.Languages))
	for _, l := range raw.Languages {
		if l.Name == "" {
			continue
		}
		p.Languages = append(p.Languages, Language{
			Name:        l.Name,
			Proficiency: l.Proficiency,
		})
	}

	// Featured
	p.Featured = make([]FeaturedItem, 0, len(raw.Featured))
	for _, f := range raw.Featured {
		if f.Title == "" {
			continue
		}
		p.Featured = append(p.Featured, FeaturedItem{
			Type:        f.Type,
			Title:       f.Title,
			Description: f.Description,
			URL:         f.URL,
		})
	}

	return p, nil
}
