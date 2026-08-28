package profile_test

import (
	"encoding/json"
	"testing"

	"github.com/tross/linkedin-profile-api/internal/linkedin"
	"github.com/tross/linkedin-profile-api/internal/profile"
)

func TestNormalize_EmptyArraysGuaranteed(t *testing.T) {
	raw := &linkedin.InternalProfile{
		Slug:             "emptyuser",
		PublicIdentifier: "emptyuser",
		FirstName:        "Empty",
		LastName:         "User",
		FullName:         "Empty User",
	}

	p, err := profile.Normalize("emptyuser", "https://www.linkedin.com/in/emptyuser/", raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}

	var jsonMap map[string]interface{}
	if err := json.Unmarshal(data, &jsonMap); err != nil {
		t.Fatal(err)
	}

	arrayFields := []string{"experience", "education", "skills", "certifications", "languages"}
	for _, f := range arrayFields {
		val, exists := jsonMap[f]
		if !exists {
			t.Errorf("field %q missing from serialized json", f)
		}
		if val == nil {
			t.Errorf("field %q serialized as null, expected []", f)
		}
		arr, ok := val.([]interface{})
		if !ok {
			t.Errorf("field %q is not a JSON array", f)
		}
		if len(arr) != 0 {
			t.Errorf("field %q has length %d, expected 0", f, len(arr))
		}
	}
}

func TestNormalize_ExperienceSorting(t *testing.T) {
	raw := &linkedin.InternalProfile{
		Slug: "user1",
		Experience: []linkedin.RawExperienceItem{
			{Title: "Old Role", Company: "Acme", StartYear: 2018, EndYear: 2020, Current: false},
			{Title: "Current Role", Company: "Hello", StartYear: 2023, Current: true},
			{Title: "Mid Role", Company: "Corp", StartYear: 2020, EndYear: 2022, Current: false},
		},
	}

	p, err := profile.Normalize("user1", "https://www.linkedin.com/in/user1/", raw)
	if err != nil {
		t.Fatal(err)
	}

	if len(p.Experience) != 3 {
		t.Fatalf("expected 3 items, got %d", len(p.Experience))
	}

	if !p.Experience[0].Current || p.Experience[0].Title != "Current Role" {
		t.Errorf("expected first item to be Current Role, got %+v", p.Experience[0])
	}
	if p.Experience[1].Title != "Mid Role" {
		t.Errorf("expected second item to be Mid Role (2020), got %+v", p.Experience[1])
	}
	if p.Experience[2].Title != "Old Role" {
		t.Errorf("expected third item to be Old Role (2018), got %+v", p.Experience[2])
	}
}

func TestNormalize_CurrentPositionNoEndDate(t *testing.T) {
	raw := &linkedin.InternalProfile{
		Slug: "user2",
		Experience: []linkedin.RawExperienceItem{
			{Title: "Dev", Company: "Tech", StartMonth: 5, StartYear: 2023, Current: true},
		},
	}

	p, err := profile.Normalize("user2", "https://www.linkedin.com/in/user2/", raw)
	if err != nil {
		t.Fatal(err)
	}

	if len(p.Experience) != 1 {
		t.Fatal("expected 1 item")
	}

	exp := p.Experience[0]
	if !exp.Current {
		t.Error("expected Current: true")
	}
	if exp.EndDate != nil {
		t.Errorf("expected EndDate: nil for current role, got %+v", exp.EndDate)
	}
	if exp.StartDate == nil || exp.StartDate.Month != 5 || exp.StartDate.Year != 2023 {
		t.Errorf("expected StartDate May 2023, got %+v", exp.StartDate)
	}
}

func TestNormalize_SkillDeduplication(t *testing.T) {
	raw := &linkedin.InternalProfile{
		Slug: "user3",
		Skills: []linkedin.RawSkillItem{
			{Name: "Go"},
			{Name: "go"},
			{Name: "Python"},
			{Name: "PYTHON"},
		},
	}

	p, err := profile.Normalize("user3", "https://www.linkedin.com/in/user3/", raw)
	if err != nil {
		t.Fatal(err)
	}

	if len(p.Skills) != 2 {
		t.Errorf("expected 2 deduplicated skills, got %d", len(p.Skills))
	}
}
