package linkedin

import (
	"encoding/json"
	"html"
	"regexp"
	"strconv"
	"strings"
)

var (
	vieweeURNRe1 = regexp.MustCompile(`(?:profileUrn=urn(?:%3A|:)li(?:%3A|:)fsd_profile(?:%3A|:)|fsd_profile(?:%3A|:))([a-zA-Z0-9_-]{5,60})`)
	vieweeURNRe2 = regexp.MustCompile(`(?:recipient=|vieweeProfileId[":\\]+)([a-zA-Z0-9_-]{5,60})`)
	vieweeURNRe3 = regexp.MustCompile(`\b(ACo[a-zA-Z0-9_-]{5,55})\b`)
	vieweeURNRe4 = regexp.MustCompile(`ref(ACo[a-zA-Z0-9_-]+)Topcard`)

	titleTagRe     = regexp.MustCompile(`<title>\s*([^|<]+)\s*\|\s*LinkedIn\s*</title>`)
	ogTitleRe      = regexp.MustCompile(`<meta\s+(?:property|name)="og:title"\s+content="([^|"]+)\s*\|\s*LinkedIn"`)
	profilePhotoRe = regexp.MustCompile(`https://media\.licdn\.com/dms/image/[^\s"'\\]+(?:profile-displayphoto|profile-framedphoto)[^\s"'\\]*`)

	flightChunkRe = regexp.MustCompile(`(?m)^([0-9a-fA-F]+):(\[.*\])$`)
	stringChildRe = regexp.MustCompile(`"children"\s*:\s*\[\s*"([^"]+)"\s*\]`)

	monthMap = map[string]int{
		"jan": 1, "feb": 2, "mar": 3, "apr": 4, "may": 5, "jun": 6,
		"jul": 7, "aug": 8, "sep": 9, "oct": 10, "nov": 11, "dec": 12,
	}
)

func DecodeTopCardFromHTML(htmlContent string) (*RawProfileTopCard, error) {
	tc := &RawProfileTopCard{}

	// Extract vieweeProfileID
	tc.VieweeProfileID = extractVieweeIDFromHTML(htmlContent)
	if tc.VieweeProfileID == "" {
		return nil, ErrProfileNotFound
	}

	// Extract Name from <title> or meta
	if m := titleTagRe.FindStringSubmatch(htmlContent); len(m) > 1 {
		tc.FullName = html.UnescapeString(strings.TrimSpace(m[1]))
	} else if m := ogTitleRe.FindStringSubmatch(htmlContent); len(m) > 1 {
		tc.FullName = html.UnescapeString(strings.TrimSpace(m[1]))
	}

	// Split name into first and last
	if tc.FullName != "" {
		parts := strings.Fields(tc.FullName)
		if len(parts) > 0 {
			tc.FirstName = parts[0]
			if len(parts) > 1 {
				tc.LastName = strings.Join(parts[1:], " ")
			}
		}
	}

	// Extract profile image
	if m := profilePhotoRe.FindString(htmlContent); m != "" {
		tc.ProfileImageURL = strings.ReplaceAll(m, `\u0026`, "&")
		tc.ProfileImageURL = html.UnescapeString(tc.ProfileImageURL)
	}

	// Extract headline and location from window.__como_rehydration__ if present
	if strings.Contains(htmlContent, "window.__como_rehydration__") {
		parseTopCardRehydration(htmlContent, tc)
	}

	return tc, nil
}

func extractVieweeIDFromHTML(h string) string {
	if m := vieweeURNRe1.FindStringSubmatch(h); len(m) > 1 {
		return m[1]
	}
	if m := vieweeURNRe2.FindStringSubmatch(h); len(m) > 1 {
		return m[1]
	}
	if m := vieweeURNRe3.FindStringSubmatch(h); len(m) > 1 {
		return m[1]
	}
	if m := vieweeURNRe4.FindStringSubmatch(h); len(m) > 1 {
		return m[1]
	}
	return ""
}

func parseTopCardRehydration(htmlContent string, tc *RawProfileTopCard) {
	idx := strings.Index(htmlContent, "window.__como_rehydration__")
	if idx == -1 {
		return
	}
	endIdx := strings.Index(htmlContent[idx:], "</script>")
	if endIdx == -1 {
		endIdx = len(htmlContent) - idx
	}
	rawBlock := htmlContent[idx : idx+endIdx]
	block := strings.ReplaceAll(rawBlock, `\"`, `"`)
	block = strings.ReplaceAll(block, `\\`, `\`)

	// Find location (e.g. "Santa Monica, California, United States")
	locRe := regexp.MustCompile(`"([A-Z][a-zA-Z\s-]+,\s+[A-Z][a-zA-Z\s-]+(?:,\s+[A-Z][a-zA-Z\s-]+)?)"`)
	for _, m := range locRe.FindAllStringSubmatch(block, -1) {
		cand := m[1]
		lower := strings.ToLower(cand)
		if strings.Contains(cand, ",") &&
			!strings.Contains(cand, ".") &&
			!strings.Contains(lower, "mode") &&
			!strings.Contains(lower, "arrow") &&
			!strings.Contains(lower, "draw") &&
			!strings.Contains(lower, "delete") &&
			!strings.Contains(lower, "navigate") &&
			!strings.Contains(lower, "chair") &&
			!strings.Contains(lower, "founder") &&
			!strings.Contains(lower, "engineer") &&
			!strings.Contains(lower, "manager") &&
			!strings.Contains(lower, "foundation") &&
			!strings.Contains(cand, "http") &&
			!strings.Contains(cand, "com.linkedin") {
			setParsedLocation(cand, tc)
			break
		}
	}

	// Find headline near name
	if tc.FullName != "" {
		nameIdx := strings.Index(block, `["`+tc.FullName+`"]`)
		if nameIdx != -1 {
			window := block[nameIdx:min(len(block), nameIdx+1200)]
			for _, sm := range stringChildRe.FindAllStringSubmatch(window, -1) {
				t := sm[1]
				if t != tc.FullName && len(t) > 3 && !strings.HasPrefix(t, "$") && !strings.Contains(t, "·") {
					tc.Headline = html.UnescapeString(t)
					break
				}
			}
		}
	}
}

func setParsedLocation(raw string, tc *RawProfileTopCard) {
	raw = strings.TrimSpace(raw)
	parts := strings.Split(raw, ",")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	if len(parts) == 3 {
		tc.City = parts[0]
		tc.Region = parts[1]
		tc.Country = parts[2]
	} else if len(parts) == 2 {
		tc.City = parts[0]
		tc.Country = parts[1]
	} else if len(parts) == 1 {
		tc.Country = parts[0]
	}
}

// DecodeFlightChunks parses newline-delimited RSC Flight stream bytes into a chunk map.
func DecodeFlightChunks(data []byte) map[string]interface{} {
	chunks := make(map[string]interface{})
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		if !strings.Contains(line, ":") {
			continue
		}
		colon := strings.Index(line, ":")
		cid := line[:colon]
		val := line[colon+1:]

		if strings.HasPrefix(val, "I[") || strings.HasPrefix(val, `"$S`) {
			continue
		}

		var parsed interface{}
		if err := json.Unmarshal([]byte(val), &parsed); err == nil {
			chunks[cid] = parsed
		}
	}
	return chunks
}

// DecodeAboutAndFeatured extracts the bio/summary and featured items from profileCardsAboveActivity payload.
func DecodeAboutAndFeatured(data []byte) (string, []RawFeaturedItem) {
	about := ""
	var featured []RawFeaturedItem

	text := string(data)

	// Extract about from chunks
	chunks := DecodeFlightChunks(data)
	for _, node := range chunks {
		s, err := json.Marshal(node)
		if err != nil {
			continue
		}
		str := string(s)
		if strings.Contains(str, "aboutSection") || strings.Contains(str, "profile-card-about") {
			for _, m := range stringChildRe.FindAllStringSubmatch(str, -1) {
				t := m[1]
				if len(t) > 20 && !strings.HasPrefix(t, "$") && !strings.Contains(t, "com.linkedin") {
					about = cleanFlightString(t)
					break
				}
			}
		}
	}

	// Extract featured items from token stream
	var extractedStrings []string
	for _, line := range strings.Split(text, "\n") {
		for _, m := range stringChildRe.FindAllStringSubmatch(line, -1) {
			t := cleanFlightString(m[1])
			if t != "" && !strings.HasPrefix(t, "$L") && !strings.HasPrefix(t, "var(--") {
				extractedStrings = append(extractedStrings, t)
			}
		}
	}

	inFeatured := false
	for i := 0; i < len(extractedStrings); i++ {
		token := extractedStrings[i]
		if token == "Featured" {
			inFeatured = true
			continue
		}
		if !inFeatured {
			continue
		}

		if token == "Link" || token == "Post" || token == "Article" || token == "Document" {
			item := RawFeaturedItem{Type: token}
			i++
			for i < len(extractedStrings) {
				next := extractedStrings[i]
				if next == "Link" || next == "Post" || next == "Article" || next == "Document" || next == "Activity" {
					i--
					break
				}
				if strings.HasPrefix(next, "http") {
					item.URL = next
				} else if item.Title == "" {
					item.Title = next
				} else if item.Description == "" {
					item.Description = next
				}
				i++
			}
			if item.Title != "" {
				featured = append(featured, item)
			}
			continue
		}

		if token == "Activity" {
			break
		}
	}

	return about, featured
}

// DecodeExperience extracts job positions from profileCardsExperienceOnly payload.
func DecodeExperience(data []byte) []RawExperienceItem {
	var items []RawExperienceItem

	text := string(data)
	if !strings.Contains(text, "profile-card-experience") && !strings.Contains(text, "Experience") {
		return items
	}

	chunks := DecodeFlightChunks(data)

	// In Flight format, experience items are represented as sequential text models.
	// We extract all strings in the chunk stream and correlate (title, company, dates, location, description).
	var extractedStrings []string
	for _, line := range strings.Split(text, "\n") {
		for _, m := range stringChildRe.FindAllStringSubmatch(line, -1) {
			t := cleanFlightString(m[1])
			if t != "" && !strings.HasPrefix(t, "$L") && !strings.HasPrefix(t, "var(--") && t != "Experience" {
				extractedStrings = append(extractedStrings, t)
			}
		}
	}

	// Parse items from the token sequence
	i := 0
	for i < len(extractedStrings) {
		token := extractedStrings[i]

		// Check if next token has " · " which typically denotes "Company · Full-time" or "Company · Part-time"
		if i+1 < len(extractedStrings) && isCompanyToken(extractedStrings[i+1]) {
			title := token
			companyFull := extractedStrings[i+1]
			company := strings.Split(companyFull, " · ")[0]
			company = strings.Split(company, " \u00b7 ")[0]

			item := RawExperienceItem{
				Title:   title,
				Company: company,
			}

			// Check for date range in token i+2
			if i+2 < len(extractedStrings) && isDateRangeToken(extractedStrings[i+2]) {
				startD, endD, curr := parseDateRangeText(extractedStrings[i+2])
				if startD != nil {
					item.StartMonth = startD.Month
					item.StartYear = startD.Year
				}
				if endD != nil {
					item.EndMonth = endD.Month
					item.EndYear = endD.Year
				}
				item.Current = curr
				i += 3

				// Check for location in next token
				if i < len(extractedStrings) && isLocationToken(extractedStrings[i]) {
					item.Location = extractedStrings[i]
					i++
				}

				// Check for description in next token
				if i < len(extractedStrings) && !isTitleOfNextJob(i, extractedStrings) && isDescriptionToken(extractedStrings[i]) {
					item.Description = extractedStrings[i]
					i++
				}

				items = append(items, item)
				continue
			}
		}
		i++
	}

	// Also look for rootUrl company logos in chunks
	for _, node := range chunks {
		b, _ := json.Marshal(node)
		s := string(b)
		if strings.Contains(s, "company-logo") && strings.Contains(s, "rootUrl") {
			// extracted if needed
		}
	}

	return items
}

// DecodeEducation extracts schools, degrees, and certifications from Part1WithoutExp.
func DecodeEducation(data []byte) ([]RawEducationItem, []RawCertificationItem) {
	var eduItems []RawEducationItem
	var certItems []RawCertificationItem

	text := string(data)
	chunks := DecodeFlightChunks(data)

	var extractedStrings []string
	for _, line := range strings.Split(text, "\n") {
		for _, m := range stringChildRe.FindAllStringSubmatch(line, -1) {
			t := cleanFlightString(m[1])
			if t != "" && !strings.HasPrefix(t, "$L") && !strings.HasPrefix(t, "var(--") && t != "Education" && t != "Licenses & certifications" && t != "Show all" {
				extractedStrings = append(extractedStrings, t)
			}
		}
	}

	i := 0
	for i < len(extractedStrings) {
		token := extractedStrings[i]

		if isSchoolToken(token) {
			item := RawEducationItem{School: token}
			if i+1 < len(extractedStrings) && isDegreeToken(extractedStrings[i+1]) {
				degreeText := extractedStrings[i+1]
				deg, fos := parseDegreeAndFOS(degreeText)
				item.Degree = deg
				item.FieldOfStudy = fos
				i++
			}
			eduItems = append(eduItems, item)
			i++
			continue
		}

		if isCertName(i, extractedStrings) {
			cert := RawCertificationItem{Name: token}
			i++
			// Next token is the issuing authority/organization
			if i < len(extractedStrings) && !strings.HasPrefix(strings.ToLower(extractedStrings[i]), "issued") && !strings.HasPrefix(strings.ToLower(extractedStrings[i]), "credential") {
				cert.Authority = extractedStrings[i]
				i++
			}
			// Skip "Issued ..." and "Credential ID ..." lines
			for i < len(extractedStrings) {
				lowerNext := strings.ToLower(extractedStrings[i])
				if strings.HasPrefix(lowerNext, "issued") || strings.HasPrefix(lowerNext, "credential id") {
					i++
				} else {
					break
				}
			}
			certItems = append(certItems, cert)
			continue
		}

		i++
	}

	_ = chunks
	return eduItems, certItems
}

func isCertName(idx int, tokens []string) bool {
	for j := idx + 1; j < len(tokens) && j <= idx+4; j++ {
		lower := strings.ToLower(tokens[j])
		if strings.HasPrefix(lower, "issued") || strings.HasPrefix(lower, "credential id") {
			return true
		}
		if isSchoolToken(tokens[j]) {
			return false
		}
	}
	return false
}

// DecodeLanguages extracts languages from Part4 payload.
func DecodeLanguages(data []byte) []RawLanguageItem {
	var items []RawLanguageItem
	text := string(data)

	var extractedStrings []string
	for _, line := range strings.Split(text, "\n") {
		for _, m := range stringChildRe.FindAllStringSubmatch(line, -1) {
			t := cleanFlightString(m[1])
			if t != "" && !strings.HasPrefix(t, "$L") && !strings.HasPrefix(t, "var(--") {
				extractedStrings = append(extractedStrings, t)
			}
		}
	}

	knownProficiencies := []string{"native", "bilingual", "professional", "elementary", "full professional", "limited working"}

	for i := 0; i < len(extractedStrings); i++ {
		lang := extractedStrings[i]
		if isKnownLanguage(lang) {
			item := RawLanguageItem{Name: lang}
			if i+1 < len(extractedStrings) {
				next := strings.ToLower(extractedStrings[i+1])
				for _, kp := range knownProficiencies {
					if strings.Contains(next, kp) {
						item.Proficiency = extractedStrings[i+1]
						i++
						break
					}
				}
			}
			items = append(items, item)
		}
	}

	return items
}

// DecodeSkills extracts skills from Part7 payload.
func DecodeSkills(data []byte) []RawSkillItem {
	var items []RawSkillItem
	text := string(data)

	var extractedStrings []string
	for _, line := range strings.Split(text, "\n") {
		for _, m := range stringChildRe.FindAllStringSubmatch(line, -1) {
			t := cleanFlightString(m[1])
			if t != "" && !strings.HasPrefix(t, "$L") && !strings.HasPrefix(t, "var(--") && t != "Skills" && t != "Show all skills" && t != "Show all" {
				extractedStrings = append(extractedStrings, t)
			}
		}
	}

	seen := make(map[string]bool)
	for _, s := range extractedStrings {
		s = strings.TrimSpace(s)
		if len(s) >= 2 && len(s) <= 40 && !seen[strings.ToLower(s)] && isSkillCandidate(s) {
			seen[strings.ToLower(s)] = true
			items = append(items, RawSkillItem{Name: s})
		}
	}

	return items
}

func isTitleOfNextJob(i int, tokens []string) bool {
	return i+1 < len(tokens) && isCompanyToken(tokens[i+1])
}

func isCompanyToken(t string) bool {
	return strings.Contains(t, " · ") || strings.Contains(t, " \u00b7 ") || strings.Contains(t, "Full-time") || strings.Contains(t, "Part-time")
}

func isDateRangeToken(t string) bool {
	return (strings.Contains(t, "Present") || strings.Contains(t, "19") || strings.Contains(t, "20")) &&
		(strings.Contains(t, " - ") || strings.Contains(t, " – ") || strings.Contains(t, " — "))
}

func isLocationToken(t string) bool {
	return (strings.Contains(t, ",") || strings.Contains(t, "Area") || strings.Contains(t, "Remote")) &&
		!strings.Contains(t, " · ") && len(t) < 80
}

func isDescriptionToken(t string) bool {
	return len(t) > 20 && !isCompanyToken(t) && !isDateRangeToken(t)
}

func isSchoolToken(t string) bool {
	lower := strings.ToLower(t)
	return strings.Contains(lower, "university") || strings.Contains(lower, "college") ||
		strings.Contains(lower, "school") || strings.Contains(lower, "institute") || strings.Contains(lower, "academy")
}

func isDegreeToken(t string) bool {
	lower := strings.ToLower(t)
	return strings.Contains(lower, "degree") || strings.Contains(lower, "bachelor") ||
		strings.Contains(lower, "master") || strings.Contains(lower, "phd") || strings.Contains(lower, "high school") ||
		strings.Contains(lower, "b.s.") || strings.Contains(lower, "b.a.") || strings.Contains(lower, "m.s.")
}

func parseDegreeAndFOS(text string) (degree, fieldOfStudy string) {
	if strings.Contains(text, ",") {
		parts := strings.SplitN(text, ",", 2)
		return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
	}
	return strings.TrimSpace(text), ""
}

func isKnownLanguage(t string) bool {
	lower := strings.ToLower(strings.TrimSpace(t))
	languages := []string{"english", "urdu", "spanish", "french", "german", "hindi", "mandarin", "chinese", "japanese", "korean", "russian", "arabic", "portuguese", "italian"}
	for _, l := range languages {
		if lower == l {
			return true
		}
	}
	return false
}

func isSkillCandidate(s string) bool {
	if strings.Contains(s, " · ") || strings.Contains(s, "http") || strings.Contains(s, "com.linkedin") {
		return false
	}
	lower := strings.ToLower(s)
	if strings.Contains(lower, "endorse") || strings.Contains(lower, "colleague") || strings.Contains(lower, "experience") {
		return false
	}
	if len(s) > 0 && (s[0] >= '0' && s[0] <= '9') {
		return false
	}
	if isSchoolToken(s) || isCompanyToken(s) || isDateRangeToken(s) {
		return false
	}
	return true
}

type parsedDate struct {
	Month int
	Year  int
}

func parseDateRangeText(text string) (startD, endD *parsedDate, current bool) {
	if strings.Contains(text, "·") {
		text = strings.TrimSpace(strings.Split(text, "·")[0])
	}
	if strings.Contains(text, "\u00b7") {
		text = strings.TrimSpace(strings.Split(text, "\u00b7")[0])
	}

	parts := regexp.MustCompile(`\s*[-–—]\s*`).Split(text, 2)
	if len(parts) != 2 {
		return nil, nil, false
	}

	startStr, endStr := strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
	current = strings.Contains(strings.ToLower(endStr), "present")

	startD = parseSingleDate(startStr)
	if !current {
		endD = parseSingleDate(endStr)
	}

	return startD, endD, current
}

func parseSingleDate(p string) *parsedDate {
	tokens := strings.Fields(p)
	if len(tokens) == 2 {
		monthPrefix := strings.ToLower(tokens[0])
		if len(monthPrefix) >= 3 {
			if m, ok := monthMap[monthPrefix[:3]]; ok {
				if y, err := strconv.Atoi(tokens[1]); err == nil {
					return &parsedDate{Month: m, Year: y}
				}
			}
		}
	} else if len(tokens) == 1 {
		if y, err := strconv.Atoi(tokens[0]); err == nil {
			return &parsedDate{Year: y}
		}
	}
	return nil
}

func cleanFlightString(s string) string {
	s = strings.ReplaceAll(s, `\n`, "\n")
	s = strings.ReplaceAll(s, `\"`, `"`)
	s = strings.ReplaceAll(s, `\u00b7`, "·")
	s = strings.ReplaceAll(s, `\u2013`, "–")
	s = strings.ReplaceAll(s, `\u2019`, "’")
	return html.UnescapeString(strings.TrimSpace(s))
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
