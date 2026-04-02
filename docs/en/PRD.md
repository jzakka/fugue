# PRD: Fugue — Cross-media Creative Curation Platform

**Date**: 2026-04-03
**Product Stage**: New (Side Project / Portfolio)
**Mascot**: A pufferfish (fugu) wearing headphones and holding a paintbrush (Fugue ≈ Fugu)

---

## 1. Summary

Fugue is a cross-media creative curation platform. While Pinterest is limited to images, Fugue lets you discover and curate music, illustration, video, writing, code, 3D, and games — all in one feed, across disciplines.

- "Pin" any creative work URL → auto-preview via OG metadata
- Organize pins into themed boards
- Personalized feed that learns your taste from clicks and pins
- Related works shown on every detail page

## 2. Contacts

| Role | Name | Responsibilities |
|------|------|-----------------|
| Planning/Development/Operations | Sanghwa Chung | Solo side project |

## 3. Background

Creative works are scattered across platforms: music on SoundCloud, illustrations on pixiv, videos on YouTube, code on GitHub. Each platform only handles its own medium, making cross-discipline discovery impossible.

Existing curation services (Pinterest, Are.na) are primarily image-focused and don't treat audio, video, or code as first-class media.

**Key insight**: Cross-media curation — discovering a SoundCloud track alongside a pixiv illustration alongside a GitHub project — is an empty space.

## 4. Objective

Enable discovery and curation of creative works across all media types in a single platform.

### Key Results (MVP)

| KR | Metric | Target |
|----|--------|--------|
| KR1 | Pinned works | 200 |
| KR2 | Boards created | 30 |
| KR3 | Registered users | 50 |
| KR4 | Feed → detail click-through rate | 20% |

## 5. Target User

People who enjoy discovering and collecting creative works across multiple disciplines.

- **Motivation**: Inspiration, reference collection, taste curation
- **Current behavior**: Browse each platform separately, browser bookmarks, links in Notion/Are.na
- **Pain points**: Scattered across platforms, no cross-discipline discovery

## 6. Solution

### Feature 1: Pin
Pin any creative work URL. Server-side OG fetch for auto-preview. Domain-based field auto-detection. Tags for style categorization.

### Feature 2: Board
Organize pins into themed collections. Public/private visibility. N:M relationship (one pin can belong to multiple boards).

### Feature 3: Recommendation Feed
Personalized feed based on user's pin/click behavior. v1: tag frequency heuristic. Roadmap: heuristic → feature store → ML.

### Feature 4: Implicit Signals
Record view, pin, and board_add events in interactions table. Used for recommendation in v1, ML training data in future versions.

### Feature 5: Related Works
Show similar works on detail page based on tag overlap. Max 10 items.

### Feature 6: Auth (Implemented)
Google OAuth, Discord OAuth. JWT-based authentication.

## 7. Release

### MVP
| Feature | Priority | Status |
|---------|----------|--------|
| Social login | P0 | Done |
| Pin CRD + OG fetch | P0 | Not started |
| Board CRUD | P0 | Not started |
| Recommendation feed (v1 heuristic) | P0 | Not started |
| Implicit signals | P0 | Not started |
| Related works | P1 | Not started |

### Post-MVP
- Feature store for recommendation
- ML-based recommendation
- Board sharing / collaborative editing
- Embedded players (SoundCloud, YouTube)
- Notifications
