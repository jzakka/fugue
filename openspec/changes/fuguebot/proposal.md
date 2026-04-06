## Why

피드가 비어있으면 큐레이션 플랫폼이 아니라 빈 게시판이다. 유저가 가입해도 볼 콘텐츠가 없으면 이탈한다. Pinterest는 Pinterestbot으로 공개 웹사이트를 크롤링해서 피드를 채운다. Fugue도 외부 창작물 플랫폼(Pixiv, SoundCloud, YouTube 등)을 자동으로 크롤링하여 콜드 스타트 문제를 해결해야 한다.

## What Changes

- Colly 기반 웹 크롤러(fuguebot) 신규 개발. API 서버와 별도 바이너리 (`cmd/bot/`)
- 플랫폼별 Source 플러그인 인터페이스 도입. 새 플랫폼 추가 = 인터페이스 구현 하나
- MVP 플러그인 2개 (Pixiv, SoundCloud 또는 YouTube)
- 외부 미디어(이미지/음원/비디오) 다운로드 → S3 미디어 버킷 저장 → 핀 자동 생성
- fuguebot 전용 시스템 계정 생성
- URL 중복 체크 (이미 핀된 URL은 skip)
- OG 텍스트에서 사전정의 태그 자동 추출 (기존 auto-suggest 로직 재사용)
- 크롤 통계를 S3 이벤트 파이프라인(Kinesis Firehose)으로 로깅
- 크롤 상태 대시보드 API + 소스 설정 API (관리자용)

## Capabilities

### New Capabilities
- `bot`: 외부 콘텐츠 크롤러 (크롤 엔진, Source 플러그인, 미디어 다운로드/S3 저장, 크롤 관리 API, 통계 로깅)

### Modified Capabilities
- `pin`: fuguebot 시스템 계정으로 핀 생성. creator_id가 봇 계정을 가리킴

## Impact

- 새 바이너리: `apps/api/cmd/bot/main.go`
- 새 패키지: `internal/bot/` (engine, source, downloader, sources/)
- DB: creators 테이블에 fuguebot 시스템 계정 시드
- 인프라: S3 미디어 버킷 (사용자가 Terraform으로 직접 구축), K8s CronJob
- 기존 코드 재사용: `internal/og/` (OG fetch), `internal/event/` (Firehose 로깅), auto-suggest 태깅 로직
- API 엔드포인트 추가: `/api/admin/bot/status`, `/api/admin/bot/sources`
- 의존성 추가: `github.com/gocolly/colly/v2`
