# PRD: Fugue — Creation-Based Collaborative Matching Platform

**Date**: 2026-03-29
**Product Stage**: New (Toy Project)
**Mascot**: A pufferfish (fugu) wearing headphones and holding a paintbrush — a wordplay on Fugue/Fugu

---

## 1. Summary

Fugue is a platform where hobby/indie creators post their work links and receive recommendations for complementary works in other fields to facilitate collaboration. Unlike commission-based platforms (VGen, Skeb, Fiverr, etc.), Fugue focuses on **free collaboration** by **matching works, not people**.

- Post a work link in 30 seconds -> auto-preview via OG metadata
- Select "What do you want to create?" -> get recommended works from other fields
- From work details, connect directly to the creator's SNS

## 2. Contacts

| Role | Name | Responsibilities |
|------|------|-----------------|
| Planning/Development/Operations | Sanghwa Chung | Solo side project |

## 3. Background

- The idea originated from wanting to turn personal compositions into YouTube MVs, but lacking illustration/video editing skills
- Reference: piapro (a platform for posting creations + sharing licenses)
- Current market: Unstructured 1:1 matching via Twitter DM is the norm
- **Key insight**: "Free + work-based collaborative matching" is something no one is doing
- All existing services are centered on "transactions (commissions)"
- Free collaboration among hobby creators still depends on Twitter DM

### Positioning Map

```
                  1:1 Commission          Team/Work Matching
                       |                        |
  Paid       VGen, Skeb, Coconala              (None)
             Fiverr, Ko-fi
                       |                        |
  Free       Piapro (Post + Derivative)        > Fugue <
```

## 4. Objective

**Help hobby creators discover works and creators in other fields most easily.**

### Key Results (KR)

| KR | Target | Measurement |
|----|--------|-------------|
| KR1 | 100 works posted | Total work count |
| KR2 | 50 creators registered | Total creator count |
| KR3 | 30 profile views via recommendation | Click-through from recommendation -> creator profile |
| KR4 | 10 SNS contact clicks | Click-through from creator profile -> SNS link |

## 5. Market Segment

**Hobby/indie creators who want collaboration opportunities over profit**

- Indie composers, hobby illustrators, video editors, etc.
- Users seeking profit are excluded from the initial target
- License: "Free to use with credit" as default

### Initial Niche

- **Music MV community** — Composers who want to create MVs need illustrators and video editors
- High collaboration demand, strong community, clear cross-field needs

### Competing Services

| Service | Model | Difference from Fugue |
|---------|-------|-----------------------|
| Piapro | Work posting + licensed derivative creation | Limited to a specific ecosystem. Posting only, no matching/recommendations |
| VGen | Paid art commissions (5%) | Commission-transaction focused. Not free collaboration |
| Skeb | One-way paid commissions | Request model with no negotiation. Not collaboration |
| Coconala | Skill marketplace (22%) | General freelancer transactions. Not specialized for creative collaboration |
| Fiverr | General freelancer market (20%) | Transaction-focused. Does not target hobby creators |
| Ko-fi | Patronage + commissions | Fan-to-creator patronage model |
| ArtStation/Behance | Portfolio showcase | Showcase only, no collaborative matching |
| BOOTH | Asset/merchandise sales | Sales-focused. pixiv ecosystem |
| nizima | Live2D model market | VTuber-specialized asset trading |

## 6. Value Proposition

| Value | Description |
|-------|-------------|
| 30-second posting | Paste a link -> auto-preview. No file upload needed |
| Work-based recommendations | Not "looking for an illustrator," but "find illustrations that fit my music" |
| Structured exploration | Filter by field, tag, project type. Better than scrolling Twitter timelines |
| Instant SNS connection | From work details -> creator profile -> Twitter/Discord direct link |

### Core Concept

**Match "works," not people.**

Instead of "Looking for an illustrator," it's "I want to find illustrations that fit my music."

## 7. Solution

### User Flow

```
┌─────────────────────────────────────────────────────────┐
│                     New User                            │
└────────────────────────┬────────────────────────────────┘
                         │
                         v
┌─────────────────────────────────────────────────────────┐
│  1. Sign Up / Log In (OAuth: Google / Twitter)          │
└────────────────────────┬────────────────────────────────┘
                         │
                         v
┌─────────────────────────────────────────────────────────┐
│  2. Create Profile                                      │
│     - Role tags (multiple selection)                    │
│     - One-line bio                                      │
│     - SNS contacts (Twitter/Discord/other)              │
└────────────────────────┬────────────────────────────────┘
                         │
                         v
┌─────────────────────────────────────────────────────────┐
│  3. Post Work (30 seconds)                              │
│     - Paste external link                               │
│       (SoundCloud/pixiv/YouTube/Twitter/other URL)      │
│     - Auto-preview via OG metadata                      │
│     - Auto-detect field + select tags                   │
│     - License: "Free to use with credit" (default)      │
└────────────────────────┬────────────────────────────────┘
                         │
                         v
┌─────────────────────────────────────────────────────────┐
│  4. "What do you want to create?"                       │
│     - Select project type                               │
│       (MV / Game / Album Art / Animation, etc.)         │
│     - Select my work to use                             │
└────────────────────────┬────────────────────────────────┘
                         │
                         v
┌─────────────────────────────────────────────────────────┐
│  5. Recommendation                                      │
│     - Show works from other fields that match tags      │
│     - Filter by required fields for project type        │
│       (e.g., MV -> Illustration + Video Editing)        │
└────────────────────────┬────────────────────────────────┘
                         │
                         v
┌─────────────────────────────────────────────────────────┐
│  6. Work Details -> Creator Profile -> SNS Contact      │
│     - View work details                                 │
│     - Visit creator profile                             │
│     - Click SNS link (Twitter/Discord) to contact       │
└─────────────────────────────────────────────────────────┘
```

### Features

#### Feature 1: Work Posting (External Link-Based) — P0

Post works via external link paste. No file upload needed.

| Item | Spec |
|------|------|
| Input | External URL (SoundCloud, pixiv, YouTube, Twitter, other) |
| Auto-preview | Fetch OG metadata (title, description, thumbnail) and generate embed |
| Field tag | Auto-detect from URL domain + manual selection |
| Style/mood tags | Manual selection (multiple) |
| License | "Free to use with credit" (default, v1 only option) |
| Validation | URL format check, OG metadata fetch failure handling |

**Supported Platform Detection**

| Platform | Detection Method | Embed Type |
|----------|-----------------|------------|
| SoundCloud | `soundcloud.com` domain | oEmbed player |
| pixiv | `pixiv.net` domain | OG image thumbnail |
| YouTube | `youtube.com`, `youtu.be` domain | oEmbed player |
| Twitter/X | `twitter.com`, `x.com` domain | OG image + text |
| Other | Any URL | OG image + text fallback |

**Field Tag Presets**

| Field | Description | Auto-detect Source |
|-------|-------------|--------------------|
| Music | Composition, arrangement, vocals, etc. | SoundCloud, YouTube |
| Illustration | Illustration, design, concept art, etc. | pixiv |
| Video | Video editing, motion graphics, etc. | YouTube |
| 3D | 3D modeling, animation, etc. | — |
| Sound | Sound effects, sound design, etc. | — |
| Vocals | Singing, voice acting, etc. | — |
| Writing | Lyrics, storytelling, worldbuilding, etc. | — |

**Style/Mood Tag Presets**

| Category | Examples |
|----------|----------|
| Mood | Emotional, Dark, Bright, Calm, Energetic |
| Genre | Electronic, Rock, Lo-fi, Orchestral, Ambient |
| Visual Style | Anime, Realistic, Pixel Art, Watercolor, Minimalist |
| Theme | Fantasy, Sci-fi, Nature, Urban, Night |

#### Feature 2: Creator Profile — P0

Creator's identity and contact information. Posted works automatically become a portfolio.

| Item | Spec |
|------|------|
| Display name | Required, 2-20 characters |
| Role tags | Multiple selection from presets (Composer, Illustrator, Video Editor, 3D Artist, Sound Designer, Vocalist, Writer) |
| One-line bio | Optional, up to 100 characters |
| SNS contacts | At least one required (Twitter, Discord, other URL) |
| Portfolio | Auto-generated from posted works (no separate upload) |
| Avatar | OAuth provider's profile image (no separate upload in v1) |

#### Feature 3: Recommendation (Work Matching) — P0

Recommend works from other fields based on project type selection and tag matching.

**Project Type Templates**

| Project Type | Required Fields | Description |
|-------------|----------------|-------------|
| MV (Music Video) | Illustration, Video | A music video combining music + visuals |
| Game | Illustration, Music, Sound, 3D | Game development requiring multiple creative fields |
| Album Art | Illustration | Album cover/jacket artwork |
| Animation | Illustration, Video, Music | Animated short or series |
| Song | Music, Vocals, Writing | Collaborative song with vocals and lyrics |
| VTuber | Illustration, 3D, Video | VTuber model + content creation |

**Recommendation Logic (v1)**

```
Input:
  - My work's field (e.g., Music)
  - My work's tags (e.g., Electronic, Night, Emotional)
  - Selected project type (e.g., MV)

Process:
  1. Look up project type template -> required fields (MV -> Illustration, Video)
  2. Exclude my own field (Music) from required fields
  3. Find works in remaining fields (Illustration, Video)
     that share at least 1 tag with my work
  4. Sort by number of matching tags (descending)
  5. Display results grouped by field

Output:
  - List of recommended works per field
  - Each work shows: thumbnail, title, tags, creator name
  - Click -> work detail -> creator profile -> SNS contact
```

**Recommendation Logic Roadmap**

| Phase | Method | Timing |
|-------|--------|--------|
| v1 | Tag matching + project type templates | MVP |
| v2 | Collaboration history/popularity weighting | After data accumulation |
| v3 | Embedding-based cross-domain similarity | Long-term |

#### Feature 4: Exploration — P0

Browse and search works and creators.

| Item | Spec |
|------|------|
| Work browsing | Grid view with thumbnails, sorted by newest |
| Creator browsing | Card view with avatar + role tags + bio, sorted by newest |
| Field filter | Single-select from field tag presets |
| Tag filter | Multi-select from style/mood tag presets |
| Search | Keyword search across work titles and creator names |
| Pagination | Infinite scroll, 20 items per page |

#### Feature 5: Authentication — P0

OAuth-based sign up and log in. No email/password in v1.

| Item | Spec |
|------|------|
| Providers | Google, Twitter (OAuth 2.0) |
| Sign up flow | OAuth -> auto-create account -> redirect to profile creation |
| Log in flow | OAuth -> redirect to home |
| Session | JWT (access token + refresh token) |
| Account deletion | Self-service from settings page |

### Assumptions

| Assumption | Validation Method |
|------------|-------------------|
| Hobby creators want to discover collaborators through works, not profiles | Track recommendation click-through rate vs. direct creator search |
| 30-second posting is fast enough to overcome the posting barrier | Track posting completion rate and time-to-post |
| Tag-based matching is sufficient for v1 recommendation quality | Qualitative feedback from early users |
| Creators are willing to be contacted via their existing SNS | Track SNS contact click rate |
| Music MV community is the right initial niche | Track field distribution of posted works |

## 8. Release Plan

### MVP (v1) — All P0

| Feature | Priority | Status |
|---------|----------|--------|
| Work Posting (External Link-Based) | P0 | Planned |
| Creator Profile | P0 | Planned |
| Recommendation (Work Matching) | P0 | Planned |
| Exploration | P0 | Planned |
| Authentication (OAuth) | P0 | Planned |

### Post-MVP — To Be Added After Validation

| Feature | Priority | Trigger |
|---------|----------|---------|
| Project Board/Hub | P1 | When recommendation -> collaboration conversion is validated |
| AI-based Recommendations | P1 | When sufficient data is accumulated (1,000+ works) |
| In-app Messaging | P2 | When SNS contact click-through rate is high but conversion is low |
| Collaboration History/Tracking | P2 | When repeat collaborations are observed |
| Payment/Settlement | P3 | Only if monetization need arises from users |
| Multimedia Storage | P3 | Only if external link limitations become a major friction |

### Success Criteria

| Metric | Target | Timeline |
|--------|--------|----------|
| Works posted | 100 | 3 months after launch |
| Creators registered | 50 | 3 months after launch |
| Profile views via recommendation | 30 | 3 months after launch |
| SNS contact clicks | 10 | 3 months after launch |
| Posting completion rate | > 70% | Ongoing |
| Recommendation click-through rate | > 20% | Ongoing |
