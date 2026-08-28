# LinkedIn Profile Normalization API

A high-performance Go HTTP service that converts any public LinkedIn profile URL into clean, structured, normalized JSON.

- **Zero browser automation**: Uses direct HTTP against LinkedIn's internal Flagship React Server Components (RSC) & Server-Driven UI (SDUI) endpoints.
- **Server-owned session**: Uses your backend `li_at` and `JSESSIONID` cookies. The caller needs no authentication.
- **Automated cookie management**: Stateful `net/http/cookiejar` synchronizes auxiliary cookies (`sdui_ver`, `lidc`, `bcookie`) across requests.
- **Concurrency-safe**: A process-wide semaphore protects the session by queuing incoming profile requests serially.

---

## Quickstart

### 1. Configure Environment
Copy `.env.example` to `.env` and insert your LinkedIn session cookies:

```bash
cp .env.example .env
```

```env
LINKEDIN_LI_AT="AQED..."
LINKEDIN_JSESSIONID='"ajax:..."'
PORT=8080
```

> **Note**: `.env` is ignored by git and will never be committed.

### 2. Run the Server

**Option A: Local Go (1.22+)**
```bash
go run ./cmd/server/
```

**Option B: Docker**
```bash
docker build -t linkedin-profile-api .
docker run -p 8080:8080 --env-file .env linkedin-profile-api
```

### 3. Test the API

**Health check:**
```bash
curl http://localhost:8080/health
```

**Fetch a profile:**
```bash
curl -X POST http://localhost:8080/v1/profiles \
  -H "Content-Type: application/json" \
  -d '{"url":"https://www.linkedin.com/in/evan-king-40072280/"}'
```

---

## API Specification

### Endpoint: `POST /v1/profiles`

#### Request
```json
{
  "url": "https://www.linkedin.com/in/evan-king-40072280/"
}
```

#### Response (200 OK)
```json
{
  "profile_url": "https://www.linkedin.com/in/evan-king-40072280/",
  "public_identifier": "evan-king-40072280",
  "name": {
    "first": "Evan",
    "last": "King",
    "full": "Evan King"
  },
  "headline": "Co-founder @ hellointerview.com",
  "about": "Helping software engineers pass technical interviews...",
  "location": {
    "city": "Santa Monica",
    "region": "California",
    "country": "United States"
  },
  "profile_image": {
    "url": "https://media.licdn.com/dms/image/v2/D5603AQGkpgd8Xb13Og/profile-displayphoto-scale_100_100/..."
  },
  "experience": [
    {
      "title": "Co-Founder",
      "company": "Hello Interview",
      "location": "Los Angeles, California, United States",
      "start_date": { "month": 5, "year": 2023 },
      "end_date": null,
      "current": true
    },
    {
      "title": "Staff Software Engineer",
      "company": "Meta",
      "location": "Greater Seattle Area",
      "start_date": { "month": 8, "year": 2017 },
      "end_date": { "month": 3, "year": 2022 },
      "current": false
    }
  ],
  "education": [
    {
      "school": "Cornell University",
      "degree": "Bachelor’s Degree",
      "field_of_study": "Computer Science"
    }
  ],
  "skills": [
    { "name": "Java" },
    { "name": "C#" }
  ],
  "certifications": [],
  "languages": []
}
```

---

## Architecture & Request Flow

```
Incoming Request: POST /v1/profiles
       │
       ▼
[ Process Semaphore (Cap: 1) ] ── Queues incoming requests; respects timeouts.
       │
       ▼
[ Stateful LinkedIn Session ]  ── Auto-manages cookies via net/http/cookiejar
       │
       ├─► 1. GET /in/{slug}/           ── SSR TopCard (Name, Headline, Location, Photo)
       ├─► 2. POST ...AboveActivity      ── About / Bio
       ├─► 3. POST ...ExperienceOnly     ── Work History (Sorted newest first)
       ├─► 4. POST ...Part1WithoutExp    ── Education & Certifications
       ├─► 5. POST ...Part4              ── Languages
       └─► 6. POST ...Part7              ── Pinned / Top Skills
       │
       ▼
[ Normalizer ] ── Returns clean JSON with guaranteed non-null empty arrays (`[]`).
```

### Why Flagship RSC?
In empirical testing, legacy Voyager REST endpoints (`/voyager/api/...`) frequently triggered anti-abuse session invalidation (`Set-Cookie: li_at=delete me; Max-Age=0`). In our controlled test, the modern Flagship RSC implementation completed 25/25 requests across 5 distinct profiles while the session remained healthy and valid.

---

## Field Completeness

| Field | Source Component | Coverage |
| :--- | :--- | :--- |
| **Name** | Initial `GET /in/{slug}/` | Full (`first`, `last`, `full`) |
| **Headline** | Initial `GET /in/{slug}/` | Full |
| **Location** | Initial `GET /in/{slug}/` | Full (`city`, `region`, `country`) |
| **Profile Image** | Initial `GET /in/{slug}/` | Full high-res CDN URL |
| **About** | `profileCardsAboveActivity` | Full summary text |
| **Experience** | `profileCardsExperienceOnly` | Full chronological list (`current: true`, `end_date: null`) |
| **Education** | `profileCardsBelowActivityPart1WithoutExp` | Full (`school`, `degree`, `field_of_study`) |
| **Certifications** | `profileCardsBelowActivityPart1WithoutExp` | Full licenses & credentials |
| **Languages** | `profileCardsBelowActivityPart4` | Full languages & proficiencies |
| **Skills** | `profileCardsBelowActivityPart7` | Pinned / Top skills from profile card |

---

## Error Handling

All errors return standard JSON responses:

```json
{
  "error": {
    "code": "INVALID_PROFILE_URL",
    "message": "Expected a LinkedIn /in/ profile URL."
  }
}
```

| HTTP Status | Error Code | Cause |
| :-: | :--- | :--- |
| `400` | `INVALID_REQUEST` / `INVALID_PROFILE_URL` | Malformed JSON or non-LinkedIn URL |
| `404` | `PROFILE_NOT_FOUND` | Profile slug does not exist or account is private |
| `415` | `INVALID_REQUEST` | Content-Type header is not `application/json` |
| `424` | `LINKEDIN_SESSION_EXPIRED` | `li_at` expired or redirected to `/login` |
| `424` | `LINKEDIN_AUTH_CHALLENGE` | LinkedIn presented a security checkpoint |
| `429` | `LINKEDIN_RATE_LIMITED` | Upstream rate limit reached |
| `502` | `LINKEDIN_UPSTREAM_ERROR` | LinkedIn upstream 5xx or connection failure |

---

## Running Tests

All unit and mock tests run **100% offline** without live credentials:

```bash
go test -v ./...
```

---

## Limitations

1. **Undocumented Protocol**: LinkedIn's internal Flagship RSC and SDUI endpoints may change without notice.
2. **Session Lifespan**: Intensive scraping from datacenter IPs may trigger account checkpoints. When triggered, the service returns `424 LINKEDIN_AUTH_CHALLENGE`.
3. **Skills Preview**: The profile card exposes top pinned skills. Extracting a candidate's complete 50+ skill history requires detail modal navigation, which is not exposed over static direct HTTP without browser virtual router state.
