## Why

Bot crawler needs to adapt to diverse website structures without manual script writing for each site. Current approach requires developer intervention for every new content source, creating a bottleneck. An AI-powered iterative script generation system can autonomously create and refine parsing scripts based on clear success criteria.

## What Changes

- AI generates Puppeteer scripts from HTML samples and target URLs
- Clear validation criteria: minimum 10 items with required `media_url` and `source_url` fields
- Iterative improvement loop: failed scripts trigger regeneration with feedback
- Failed scripts saved to `/tmp` for debugging and learning
- Generalized prompts work across different website types (no site-specific hardcoding)

## Capabilities

### New Capabilities
<!-- No new capabilities are introduced -->

### Modified Capabilities
- `bot-pioneer-crawler`: AI 스크립트 생성에 반복적 개선 루프 추가 (실패 시 피드백과 함께 재생성)
- `bot-script-lifecycle`: 검증 기준 명확화 (최소 10개 아이템, media_url + source_url 필수)

## Impact

- **New**: `internal/bot/ai/` - AI client for script generation
- **Modified**: `internal/bot/interfaces.go` - AIClient interface with GenerateScript method accepting maxIterations
- **Modified**: `internal/bot/source.go` - RawItem struct with proper JSON tags
- **New**: Validation logic ensuring data quality before accepting generated scripts
- **External**: Requires `codex-cli` or similar AI command-line tool
