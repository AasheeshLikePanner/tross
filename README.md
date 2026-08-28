# LinkedIn Profile Normalization API

A lightweight, zero-browser Go service that converts any public LinkedIn profile URL into clean, structured JSON using LinkedIn's internal Flagship React Server Components (RSC) protocol.

- **Zero Browser Overhead**: Uses direct HTTP only (no Puppeteer, Playwright, or Selenium).
- **Server-Owned Authentication**: Backed by a single server-owned LinkedIn session.
- **Stateful Session**: Auto-manages dynamic cookies (`JSESSIONID`, `sdui_ver`, `lidc`) via `net/http/cookiejar`.
- **Concurrency-Safe**: Process-wide semaphore (capacity 1) serializes requests to protect account health.

---

## Quickstart

### 1. Configure Environment
```bash
cp .env.example .env
```
Add your LinkedIn cookies to `.env`:
```env
LINKEDIN_LI_AT="AQED..."
LINKEDIN_JSESSIONID="ajax:..."
PORT=8080
```

### 2. Run the Service

**Local Go (1.22+)**
```bash
go run ./cmd/server/
```

**Docker**
```bash
docker build -t linkedin-profile-api .
docker run -p 8080:8080 --env-file .env linkedin-profile-api
```

---

## API Usage

### `POST /v1/profiles`

```bash
curl -X POST http://localhost:8080/v1/profiles \
  -H "Content-Type: application/json" \
  -d '{"url":"https://www.linkedin.com/in/meetcshah19/"}'
```

#### Response (200 OK)
```json
{
  "profile_url": "https://www.linkedin.com/in/meetcshah19/",
  "public_identifier": "meetcshah19",
  "name": {
    "first": "Meet",
    "last": "Shah",
    "full": "Meet Shah"
  },
  "headline": "Co-founder @ Tross",
  "location": {
    "city": "San Francisco",
    "region": "California",
    "country": "United States"
  },
  "profile_image": {
    "url": "https://media.licdn.com/dms/image/..."
  },
  "featured": [
    {
      "type": "Link",
      "title": "Get APIs for any EHR and Payer Portals",
      "url": "https://ontross.com/"
    }
  ],
  "experience": [
    {
      "title": "Co-Founder and CTO",
      "company": "Tross",
      "start_date": { "month": 6, "year": 2025 },
      "end_date": null,
      "current": true
    }
  ],
  "education": [
    {
      "school": "Indian Institute of Technology, Roorkee",
      "degree": "Bachelor of Technology - BTech",
      "field_of_study": "Computer Science"
    }
  ],
  "skills": [
    { "name": "Java" },
    { "name": "Python" }
  ],
  "certifications": [
    {
      "name": "CSAW'21 Embedded Security Challenge Finalist India",
      "authority": "CSAW Cybersecurity Games & Conference"
    }
  ],
  "languages": []
}
```

---

## Architecture Flow

```
POST /v1/profiles
      │
      ▼
[ Concurrency Semaphore (1) ] ── Queues concurrent requests serially
      │
      ▼
[ LinkedIn Client ] ── Replays Flagship RSC calls via stateful cookiejar
      ├─► GET  /in/{slug}/           ── TopCard (Name, Headline, Location, Photo)
      ├─► POST ...AboveActivity      ── About & Featured Items
      ├─► POST ...ExperienceOnly     ── Work History
      ├─► POST ...Part1WithoutExp    ── Education & Certifications
      ├─► POST ...Part4              ── Languages
      └─► POST ...Part7              ── Skills
      │
      ▼
[ Normalizer ] ── Returns clean JSON with guaranteed non-null empty arrays (`[]`)
```

---

## Testing

All tests run **100% offline** with mock fixtures (no live credentials needed):

```bash
go test -v ./...
```
