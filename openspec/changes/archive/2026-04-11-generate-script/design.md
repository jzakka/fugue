## Context

Bot crawler currently requires manually written parsers for each content source. As Fugue expands to cover more platforms (Pixiv, SoundCloud, ArtStation, Behance, etc.), this approach doesn't scale. We need autonomous script generation that can adapt to different website structures without developer intervention for each new site.

Constraints:
- AI must work with raw HTML samples (no browser required during generation)
- Generated scripts run in Node.js with Puppeteer for execution
- Must handle both static and dynamic (JavaScript-rendered) content
- No hardcoded site-specific logic in prompts

## Goals / Non-Goals

**Goals:**
- AI generates valid Puppeteer scripts from HTML samples
- Clear, measurable success criteria (min 10 items, required fields)
- Automatic retry with feedback when validation fails
- Works across diverse content types (images, audio, video)
- Failed scripts saved for debugging and prompt improvement

**Non-Goals:**
- Real-time browser control during generation (AI works with static HTML)
- Training custom ML models (uses existing LLM via CLI)
- Guaranteed success on first try (iterative improvement is expected)
- Handling CAPTCHAs or anti-bot measures (execution concern, not generation)

## Decisions

### Decision 1: Iterative Generation with Validation Loop

**Choice**: Generate → Execute → Validate → Retry (if failed) up to N iterations

**Rationale**: AI outputs are non-deterministic. Single-shot generation might produce DOM scraping (5-15 items) instead of JSON parsing (50+ items). Validation loop with feedback guides AI toward better strategies.

**Alternatives Considered**:
- One-shot generation: Too unreliable, success depends on luck
- Manual prompt tuning per site: Defeats purpose of automation
- Training site-specific models: Too slow, requires labeled data

**Trade-off**: Slower first run (multiple AI calls) but higher quality and consistency.

### Decision 2: Clear Success Criteria (10+ Items with Required Fields)

**Choice**: `validCount >= 10` where each item has non-empty `media_url` AND `source_url`

**Rationale**: Forces AI toward comprehensive extraction methods (JSON parsing over DOM scraping). Title is optional as some sources lack metadata.

**Alternatives Considered**:
- Lower threshold (5 items): Too easy, accepts incomplete strategies
- Require title: Many sources lack titles (thumbnails, audio tracks)
- Percentage-based (80% valid): Unclear semantics, harder to reason about

**Trade-off**: May require multiple iterations for complex sites, but ensures data quality.

### Decision 3: Save Failed Scripts to `/tmp`

**Choice**: Write failed scripts to temporary storage (implementation: `/tmp/fugue-failed-script-iter{N}-{domain}.js`)

**Rationale**: 
- Debugging: Inspect what AI tried before success
- Prompt optimization: Include file reference (not full code) in feedback
- Learning: Identify common failure patterns across sites

**Alternatives Considered**:
- Include full script in prompt: Bloats prompt, wastes tokens
- Discard failed scripts: Lose debugging information
- Store in database: Overkill for ephemeral debug data

**Trade-off**: Requires tmp cleanup, but provides valuable debugging artifacts.

**Implementation Note**: Current implementation uses `/tmp` directory with naming pattern `fugue-failed-script-iter{N}-{domain}.js`. This can be changed to any persistent storage without affecting external behavior.

### Decision 4: Generalized Prompts (No Site-Specific Code)

**Choice**: Prompts describe extraction strategies (JSON parsing priority, DOM fallback) without hardcoded selectors or site names

**Rationale**: Same prompt must work for Pixiv (art), SoundCloud (audio), YouTube (video). Site-specific code in prompts defeats automation goal.

**Alternatives Considered**:
- Site-specific prompt templates: Requires manual work for each site
- Include example code: AI copies examples literally, fails on variations
- Separate prompts per content type: Increases maintenance burden

**Trade-off**: AI might take longer to discover optimal approach, but system remains scalable.

## Risks / Trade-offs

**[Risk]** AI generates working script on first try, then fails on retry due to non-determinism  
→ **Mitigation**: Accept first success immediately, don't retry needlessly

**[Risk]** Max iterations exhausted, still failing  
→ **Mitigation**: Return script with highest valid item count among all attempts. Only fail if best attempt extracted 0 items.

**[Risk]** Website structure changes after script generation  
→ **Mitigation**: Detection happens at execution time, triggers regeneration (future work)

**[Risk]** AI hallucinates valid-looking but incorrect scripts  
→ **Mitigation**: Validation runs actual execution, not just syntax check

**[Risk]** Generated code executes arbitrary operations  
→ **Mitigation**: Prompt explicitly forbids child processes, filesystem access, env vars, external packages, network requests beyond target URL. Timeout constraints enforced.

**[Trade-off]** Multiple AI calls increase latency and cost  
→ **Accepted**: Quality over speed for one-time script generation

**[Trade-off]** `/tmp` files accumulate over time  
→ **Future**: Add cleanup job or size-based rotation

## Example: Iterative Generation Flow

```
Iteration 1: Generate script from HTML sample
├─ AI generates script (approach: DOM scraping)
├─ Execute script → Extract 6 items
└─ Validate → FAIL (6 < 10 required)
    └─ Save to /tmp/fugue-failed-script-iter1-pixiv.net.js
    └─ Feedback: "Only 6 valid items (need at least 10)"

Iteration 2: Regenerate with feedback
├─ Prompt includes: "Previous attempt extracted only 6 items"
├─ AI generates script (approach: JSON parsing from __NEXT_DATA__)
├─ Execute script → Extract 66 items
└─ Validate → PASS (66 >= 10 required)
    └─ Return successful script

Result: Script saved, ready for production use
Cost: 2 AI calls (1 failed, 1 success)
```

**Key Characteristics**:
- Non-deterministic: AI may choose different approaches each time
- Self-correcting: Feedback guides AI toward better strategies
- Quality-focused: Accepts higher cost for reliable extraction
- Debugging-friendly: Failed attempts preserved for analysis
