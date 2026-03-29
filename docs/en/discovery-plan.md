# Discovery Plan: Fugue — Creation-Based Collaborative Matching Platform

**Date**: 2026-03-29
**Product Stage**: New (Toy Project)
**Discovery Question**: What is the MVP that enables hobby creators to easily discover works in other fields needed for collaboration, based on their own creations?

---

## Background

- The idea originated from wanting to turn personal compositions into YouTube MVs, but lacking illustration/video editing skills
- Reference: piapro (a platform for posting creations + sharing licenses)
- Current market: Unstructured 1:1 matching via Twitter DM is the norm

## Target Users

**Hobby/indie creators who want collaboration opportunities over profit**

- Indie composers, hobby illustrators, video editors, etc.
- Users seeking profit are excluded from the initial target
- License: "Free to use with credit" as default

## Market Research

### Positioning Map

```
                  1:1 Commission          Team/Work Matching
                       |                        |
  Paid       VGen, Skeb, Coconala              (None)
             Fiverr, Ko-fi
                       |                        |
  Free       Piapro (Post + Derivative)        > Fugue <
```

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

### Key Insights

- **"Free + work-based collaborative matching" is something no one is doing**
- Even Piapro is about posting + license sharing, not matching/recommendations
- All existing services are centered on "transactions (commissions)"
- Free collaboration among hobby creators still depends on Twitter DM

## Core Concept

**Match "works," not people.**

Instead of "Looking for an illustrator," it's "I want to find illustrations that fit my music."

### Core Flow

```
1. Post (30 seconds)
   Paste a work link (SoundCloud/pixiv/YouTube/Twitter)
   -> Auto-preview via OG metadata
   -> Auto-detect field + select tags

2. "What do you want to create?"
   Select project type (MV / Game / Album Art / Animation, etc.)
   -> Select my work

3. Recommendation
   Recommend works from other fields needed for the project type, based on tags
   -> From work details, directly link to creator profile/SNS
```

## MVP Scope

### Included

#### 1. Work Posting (External Link-Based)
- Paste external link -> auto-generate embed/OG preview
- Supported platforms: SoundCloud, pixiv, YouTube, Twitter, other URLs
- Field tags: Music, Illustration, Video, 3D, Sound, etc.
- Style/mood tags: Emotional, Night, Electronic, Fantasy, etc.
- License: "Free to use with credit" as default

#### 2. Creator Profile
- Role tags (multiple selection)
- One-line bio
- SNS contacts (Twitter/Discord/other)
- Posted works automatically become a portfolio

#### 3. Recommendation (Work Matching)
- "What do you want to create?" -> Select project type
- Required-field templates per project type (MV -> Illustration + Video Editing)
- Recommend works from other fields with matching tags
- Work details -> Creator profile -> Direct link to SNS contacts

#### 4. Exploration
- Filter by field, by tag
- Search works / Search creators

### Excluded (To Be Added After Validation)

| Feature | Reason for Exclusion | Alternative |
|---------|---------------------|-------------|
| Project Board/Hub | Posting + recommendation validation comes first | Recommendation -> Contact -> Collaborate externally |
| AI-based Recommendations | Insufficient initial data | Tag matching + type-based templates |
| Payment/Settlement | Free collaboration target | Removed |
| Multimedia Storage | Infrastructure cost | Replaced with external link embeds |
| DM/Chat | Over-engineering | Twitter DM/Discord |

### Recommendation Logic Roadmap

| Phase | Method | Timing |
|-------|--------|--------|
| v1 | Tag matching + project type templates | MVP |
| v2 | Collaboration history/popularity weighting | After data accumulation |
| v3 | Embedding-based cross-domain similarity | Long-term |

## Cold Start Strategy

1. **Minimize posting barriers** — Paste a link + select tags, done in 30 seconds
2. **Gather one side (supply) first** — Posting works alone has value (portfolio + exposure opportunity)
3. **Be the first user yourself** — Post your own compositions, then personally test the "Create MV" recommendation flow
4. **Target niche communities** — Start with the music MV community
5. **Maintain existing SNS activity** — External link-based, so zero switching cost

## Key Risks

| Risk | Description | Mitigation Strategy |
|------|-------------|---------------------|
| Lack of switching motivation | "Why should I post my link here?" | Collaboration opportunities via recommendations = unique value not found on existing SNS |
| Cold Start | Recommendations won't work if there aren't enough works | Minimize posting barriers + gather one side first |
| Tag matching quality | Can "fitting" works be found by tags alone? | Validate in v1, then enhance recommendations |
| Recommendation-to-collaboration conversion | Will recommendations actually lead to contact? | Direct link to creator SNS from work details |
