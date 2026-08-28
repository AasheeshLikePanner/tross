package linkedin_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/tross/linkedin-profile-api/internal/linkedin"
)

func TestDecodeTopCardFromHTML(t *testing.T) {
	htmlSample := `<!DOCTYPE html><html><head>
<title>Evan King | LinkedIn</title>
<meta property="og:title" content="Evan King | LinkedIn">
<a href="/messaging/compose/?profileUrn=urn%3Ali%3Afsd_profile%3AACoAABExRcABJ3yzKC4MRWrv8iATSQK7FCXah9Y&amp;recipient=ACoAABExRcABJ3yzKC4MRWrv8iATSQK7FCXah9Y">Message</a>
<img src="https://media.licdn.com/dms/image/v2/D5603AQGkpgd8Xb13Og/profile-displayphoto-scale_100_100/0?e=123" />
<script>
window.__como_rehydration__ = [
  "82:[\"$\",\"p\",null,{\"children\":[\"Evan King\"]}]\n",
  "83:[\"$\",\"$L92\",null,{\"textProps\":{\"children\":[\"Co-founder @ hellointerview.com\"]}}]\n",
  "84:\"Santa Monica, California, United States\"\n"
];
</script>
</head><body></body></html>`

	tc, err := linkedin.DecodeTopCardFromHTML(htmlSample)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if tc.VieweeProfileID != "ACoAABExRcABJ3yzKC4MRWrv8iATSQK7FCXah9Y" {
		t.Errorf("expected vieweeProfileID ACoAABExRcABJ3yzKC4MRWrv8iATSQK7FCXah9Y, got %q", tc.VieweeProfileID)
	}
	if tc.FullName != "Evan King" || tc.FirstName != "Evan" || tc.LastName != "King" {
		t.Errorf("name mismatch: %+v", tc)
	}
	if tc.Headline != "Co-founder @ hellointerview.com" {
		t.Errorf("headline mismatch: %q", tc.Headline)
	}
	if tc.City != "Santa Monica" || tc.Region != "California" || tc.Country != "United States" {
		t.Errorf("location mismatch: %q, %q, %q", tc.City, tc.Region, tc.Country)
	}
	if tc.ProfileImageURL == "" {
		t.Error("expected profile image URL, got empty")
	}
}

func TestDecodeTopCardFromHTML_Unicode(t *testing.T) {
	htmlSample := `<!DOCTYPE html><html><head>
<title>José García &amp; Co. | LinkedIn</title>
<a href="/messaging/compose/?profileUrn=urn%3Ali%3Afsd_profile%3AACoAAUnicode1234567890">Message</a>
</head></html>`

	tc, err := linkedin.DecodeTopCardFromHTML(htmlSample)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tc.FullName != "José García & Co." {
		t.Errorf("expected decoded unescaped name, got %q", tc.FullName)
	}
	if tc.FirstName != "José" {
		t.Errorf("expected first name José, got %q", tc.FirstName)
	}
}

func TestDecodeExperience_Fixture(t *testing.T) {
	path := filepath.Join("testdata", "experience.bin")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("testdata/experience.bin not found: %v", err)
	}

	items := linkedin.DecodeExperience(data)
	if len(items) == 0 {
		t.Fatal("expected experience items, got 0")
	}

	// Verify first item is Hello Interview (Current role)
	foundCurrent := false
	foundPast := false
	for _, item := range items {
		if item.Current && item.EndYear == 0 {
			foundCurrent = true
		}
		if !item.Current && item.EndYear > 0 {
			foundPast = true
		}
	}

	if !foundCurrent {
		t.Error("expected at least one current experience item with Current: true")
	}
	if !foundPast {
		t.Error("expected at least one past experience item with EndYear > 0")
	}
}

func TestDecodeEducation_Fixture(t *testing.T) {
	path := filepath.Join("testdata", "part1.bin")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("testdata/part1.bin not found: %v", err)
	}

	edu, _ := linkedin.DecodeEducation(data)
	if len(edu) == 0 {
		t.Fatal("expected education items, got 0")
	}

	foundCornell := false
	for _, e := range edu {
		if e.School == "Cornell University" {
			foundCornell = true
			if e.Degree != "Bachelor’s Degree" {
				t.Errorf("expected degree Bachelor’s Degree, got %q", e.Degree)
			}
			if e.FieldOfStudy != "Computer Science" {
				t.Errorf("expected field of study Computer Science, got %q", e.FieldOfStudy)
			}
		}
	}

	if !foundCornell {
		t.Error("expected to find Cornell University in education")
	}
}

func TestDecodeLanguages(t *testing.T) {
	sample := `0:["$","div",null,{"children":[["$","p",null,{"children":["English"]}],["$","p",null,{"children":["Native or bilingual proficiency"]}],["$","p",null,{"children":["Urdu"]}],["$","p",null,{"children":["Professional working proficiency"]}]]}]`

	langs := linkedin.DecodeLanguages([]byte(sample))
	if len(langs) != 2 {
		t.Fatalf("expected 2 languages, got %d", len(langs))
	}
	if langs[0].Name != "English" || langs[0].Proficiency != "Native or bilingual proficiency" {
		t.Errorf("language 0 mismatch: %+v", langs[0])
	}
	if langs[1].Name != "Urdu" || langs[1].Proficiency != "Professional working proficiency" {
		t.Errorf("language 1 mismatch: %+v", langs[1])
	}
}

func TestDecodeSkills(t *testing.T) {
	sample := `0:["$","div",null,{"children":[["$","span",null,{"children":["Go"]}],["$","span",null,{"children":["Python"]}],["$","span",null,{"children":["Go"]}]]}]`

	skills := linkedin.DecodeSkills([]byte(sample))
	if len(skills) != 2 {
		t.Fatalf("expected 2 deduplicated skills, got %d", len(skills))
	}
	if skills[0].Name != "Go" || skills[1].Name != "Python" {
		t.Errorf("skills mismatch: %+v", skills)
	}
}

func TestDecode_MalformedAndEmpty(t *testing.T) {
	// Should not panic on garbage input
	garbage := []byte("!@#$%^&*()_+=-[]{};':\",.<>/?\\|`~")
	if about, _ := linkedin.DecodeAboutAndFeatured(garbage); about != "" {
		t.Errorf("expected empty about for garbage input, got %q", about)
	}
	if exp := linkedin.DecodeExperience(garbage); len(exp) != 0 {
		t.Errorf("expected empty exp, got %d", len(exp))
	}
	if edu, certs := linkedin.DecodeEducation(garbage); len(edu) != 0 || len(certs) != 0 {
		t.Errorf("expected empty edu/certs, got %d / %d", len(edu), len(certs))
	}
	if langs := linkedin.DecodeLanguages(garbage); len(langs) != 0 {
		t.Errorf("expected empty langs, got %d", len(langs))
	}
	if skills := linkedin.DecodeSkills(garbage); len(skills) != 0 {
		t.Errorf("expected empty skills, got %d", len(skills))
	}
}
