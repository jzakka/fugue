# Discovery Plan: Fugue — Cross-media Creative Curation Platform

**Date**: 2026-04-03
**Product Stage**: New (Side Project / Portfolio)
**Discovery Question**: What is the MVP that enables discovery and curation of creative works across all media types in one place?

---

## Background

- Creative works are scattered across platforms: music on SoundCloud, illustrations on pixiv, videos on YouTube, code on GitHub
- Each platform only handles its own medium, making cross-discipline discovery impossible
- Finding "an illustration that fits this music" requires browsing multiple platforms
- Existing curation services (Pinterest, Are.na) are image-focused. Music/code are second-class citizens

## Target Users

**People who enjoy discovering and collecting creative works across multiple disciplines**

- Creators collecting inspiration, designers organizing references, taste curators
- Any discipline: music, illustration, video, writing, code, 3D, games, sound
- Initial niche: Creators interested in music MV production (clearest cross-discipline pattern)

## Market Research

### Positioning Map

```
              Images only        Cross-media
                │                    │
  With recs   Pinterest             ▶ Fugue ◀
                │                    │
  No recs     Are.na, Raindrop     (empty)
```

### Reference Services

| Service | Model | Difference from Fugue |
|---------|-------|----------------------|
| Pinterest | Image curation (pin + board + recommendation) | Image-only. Cannot pin music/code |
| Are.na | Research/archiving curation | No recommendation. Niche community |
| Raindrop.io | Bookmark manager | Organization tool, not curation/recommendation |
| Piapro | Creative posting + license sharing | Limited to Vocaloid ecosystem. No recommendation |
| SoundCloud/pixiv/YouTube | Single-discipline platforms | Own discipline only. No cross-discovery |

### Key Insight

- **Cross-media curation + recommendation is an empty space**
- Extend Pinterest's proven model (pin + board + recommendation) to all creative media
- The core value is discovering creative works across disciplines in one place

## Core Concept

**Pinterest for all creative media.**

Music, illustration, video, writing, and code coexist in a single feed. The more you click, the more it learns your taste and surfaces better works.

### Core Flow

```
1. Pin (30 seconds)
   Paste URL (SoundCloud/pixiv/YouTube/GitHub/anywhere)
   → Auto-preview via OG metadata
   → Auto-detect field + select tags

2. Board
   Organize pins into themed collections
   → "Dreamy indie music", "Cyberpunk illustrations", "Creative coding"

3. Discover
   Explore new works in recommendation-based feed
   → Click/pin behavior reflected as taste signals
   → Related works shown on detail pages
```

## MVP Scope

### Included

#### 1. Pin
- External URL curation. Not ownership claim, but curation.
- OG auto-preview + domain-based field detection
- Style tags (1-5 per pin)
- Create, Read, Delete only (no Update)

#### 2. Board
- Organize pins into themed collections
- Public/private visibility
- One pin can belong to multiple boards

#### 3. Recommendation Feed
- Tag frequency heuristic (v1)
- Cold start: latest-first when < 10 pins
- Batch/Redis caching

#### 4. Implicit Signals
- Record view/pin/board_add events
- Input data for recommendation engine

#### 5. Related Works
- Show tag-similar works on detail page

#### 6. Auth (Implemented)
- Google OAuth, Discord OAuth

### Excluded (Post-validation)

| Feature | Reason | Alternative |
|---------|--------|-------------|
| Collaboration matching | Focus on curation | Happens organically outside platform |
| ML-based recommendation | Insufficient initial data | Start with tag heuristic |
| Embedded players | OG preview sufficient | External link navigation |
| DM/Chat | Over-engineering | External SNS |
| File hosting | Infrastructure cost | External link-based |

### Recommendation Roadmap

| Phase | Method | Timing |
|-------|--------|--------|
| v1 | Tag frequency heuristic | MVP |
| v2 | Feature store | After data accumulation |
| v3 | ML model training | On feature store foundation |

## Cold Start Strategy

1. **Minimize pin friction** — paste URL + tags, 30 seconds
2. **Be the first user** — pin favorite works yourself to populate the feed
3. **Niche community** — start with music MV creators
4. **Zero switching cost** — external link-based, existing SNS activity stays
5. **Board sharing** — share board links on SNS for inbound

## Key Risks

| Risk | Description | Mitigation |
|------|-------------|------------|
| Motivation gap | "Why post links here?" | Board organization + recommendation = better than browser bookmarks |
| Cold Start | Not enough works for recommendation | Minimize friction + be first user |
| Tag matching quality | Can tags alone match taste? | Validate in v1, evolve to feature store/ML |
| OG fetch limitations | Some platforms have poor OG metadata | Manual input fallback |
