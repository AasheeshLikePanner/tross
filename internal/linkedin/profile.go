package linkedin

import (
	"context"
	"fmt"
	"log/slog"
)

func (c *Client) FetchProfile(ctx context.Context, slug string) (*InternalProfile, error) {
	if err := c.AcquireFetchLock(ctx); err != nil {
		return nil, err
	}
	defer c.ReleaseFetchLock()

	if !c.session.HasAuth() {
		return nil, ErrSessionExpired
	}

	referer := fmt.Sprintf("%s/in/%s/", c.baseURL, slug)

	// Step 1: Initial HTML document request
	_, topCard, err := c.FetchProfileDocument(ctx, slug)
	if err != nil {
		return nil, err
	}
	if topCard == nil || topCard.VieweeProfileID == "" {
		return nil, ErrProfileNotFound
	}

	profile := &InternalProfile{
		Slug:             slug,
		PublicIdentifier: slug,
		VieweeProfileID:  topCard.VieweeProfileID,
		FirstName:        topCard.FirstName,
		LastName:         topCard.LastName,
		FullName:         topCard.FullName,
		Headline:         topCard.Headline,
		City:             topCard.City,
		Region:           topCard.Region,
		Country:          topCard.Country,
		ProfileImageURL:  topCard.ProfileImageURL,
		Experience:       []RawExperienceItem{},
		Education:        []RawEducationItem{},
		Certifications:   []RawCertificationItem{},
		Languages:        []RawLanguageItem{},
		Skills:           []RawSkillItem{},
	}

	vieweeID := topCard.VieweeProfileID

	// Step 2: About section via profileCardsAboveActivity
	if data, err := c.FetchComponent(ctx, CompAbove, slug, vieweeID, referer); err == nil {
		profile.About = DecodeAbout(data)
		slog.Info("component fetch succeeded", "component", "about", "bytes", len(data))
	} else {
		slog.Warn("component fetch failed", "component", "about", "err", err)
	}

	// Step 3: Experience section via profileCardsExperienceOnly
	if data, err := c.FetchComponent(ctx, CompExp, slug, vieweeID, referer); err == nil {
		profile.Experience = DecodeExperience(data)
		slog.Info("component fetch succeeded", "component", "experience", "items", len(profile.Experience), "bytes", len(data))
	} else {
		slog.Warn("component fetch failed", "component", "experience", "err", err)
	}

	// Step 4: Education & Certifications via Part1WithoutExp
	if data, err := c.FetchComponent(ctx, CompPart1, slug, vieweeID, referer); err == nil {
		edu, certs := DecodeEducation(data)
		if len(edu) > 0 {
			profile.Education = edu
		}
		if len(certs) > 0 {
			profile.Certifications = certs
		}
		slog.Info("component fetch succeeded", "component", "education_certs", "edu", len(edu), "certs", len(certs))
	} else {
		slog.Warn("component fetch failed", "component", "education_certs", "err", err)
	}

	// Step 5: Languages via Part4
	if data, err := c.FetchComponent(ctx, CompPart4, slug, vieweeID, referer); err == nil {
		langs := DecodeLanguages(data)
		if len(langs) > 0 {
			profile.Languages = langs
		}
		slog.Info("component fetch succeeded", "component", "languages", "items", len(langs))
	} else {
		slog.Warn("component fetch failed", "component", "languages", "err", err)
	}

	// Step 6: Skills via Part7
	if data, err := c.FetchComponent(ctx, CompPart7, slug, vieweeID, referer); err == nil {
		skills := DecodeSkills(data)
		if len(skills) > 0 {
			profile.Skills = skills
		}
		slog.Info("component fetch succeeded", "component", "skills", "items", len(skills))
	} else {
		slog.Warn("component fetch failed", "component", "skills", "err", err)
	}

	return profile, nil
}
