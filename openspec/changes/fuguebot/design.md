## Context

Fugue 피드는 유저가 직접 핀을 만들어야만 콘텐츠가 생긴다. 가입 즉시 빈 피드를 보는 유저는 이탈한다. Pinterest는 Pinterestbot으로 공개 웹사이트를 크롤링해 피드를 채운다. Fugue도 동일한 접근이 필요하다.

핀 모델이 미디어 필수로 피벗됨 (2026-04-06). fuguebot은 외부 미디어를 다운로드하여 S3에 저장한 후 핀을 생성해야 한다.

기존 인프라: Go + Chi + sqlc, PostgreSQL, Redis, Kinesis Firehose → S3 이벤트 파이프라인, EKS + Terraform.

## Goals / Non-Goals

**Goals:**
- 외부 창작물 플랫폼을 자동으로 크롤링하여 피드를 채운다
- 플랫폼 추가가 인터페이스 구현 하나로 가능한 플러그인 아키텍처
- robots.txt 존중, rate limit 준수하는 예의 바른 크롤러
- 미디어 다운로드 → S3 저장 → 핀 생성 파이프라인
- 크롤 상태 모니터링 및 소스 동적 관리

**Non-Goals:**
- 실시간 스트리밍 크롤링 (배치/cron 기반)
- 크롤링한 콘텐츠의 저작권 검증
- 유저 대면 크롤러 설정 UI (관리자 API만)
- 미디어 변환/리사이징 (원본 그대로 저장)

## Decisions

### D1: Colly를 크롤링 프레임워크로 사용
**선택:** github.com/gocolly/colly/v2
**대안:** 직접 구현 (net/http + goquery), chromedp (headless Chrome)
**근거:** Colly는 Go 생태계 표준 크롤러 (23k+ stars). robots.txt, rate limit, 병렬 크롤링, 캐싱 내장. 직접 구현은 학습엔 좋지만 boilerplate가 많고, chromedp는 JS 렌더링이 필요한 사이트용인데 MVP에선 불필요.

### D2: API 서버와 별도 바이너리
**선택:** `cmd/bot/main.go` — 독립 프로세스
**대안:** API 서버 내 goroutine, 별도 마이크로서비스
**근거:** 크롤러는 CPU/네트워크 바운드 작업. API 서버와 리소스를 공유하면 응답 지연. K8s CronJob으로 스케줄링하면 실행 안 할 때 리소스 0. 별도 마이크로서비스는 오버엔지니어링.

### D3: Source interface 기반 플러그인
**선택:** Go interface로 플랫폼별 Source 정의
**대안:** 설정 파일 기반 크롤 룰, 범용 OG scraper
**근거:** 플랫폼마다 HTML 구조가 다르므로 설정 파일로는 한계. Go interface는 타입 안전하고 테스트 용이. 새 플랫폼 = 새 struct 하나.

```go
type Source interface {
    Name() string
    SeedURLs() []string
    Configure(c *colly.Collector)
    Extract(e *colly.HTMLElement) (*RawItem, error)
}

type RawItem struct {
    Title       string
    Description string
    MediaURL    string   // 다운로드할 미디어 URL
    SourceURL   string   // 원본 페이지 URL
    Tags        []string // 자동 추출 태그 (선택)
}
```

### D4: 미디어 다운로드 → S3 저장
**선택:** fuguebot이 직접 HTTP GET → S3 PutObject
**대안:** 미디어 URL만 저장하고 프론트에서 직접 로드
**근거:** 외부 미디어 URL은 언제든 깨질 수 있음 (hotlink 방지, 삭제). 핀 모델이 미디어 필수이므로 자체 저장 필수. S3에 저장하면 CDN 붙이기도 쉬움.

### D5: 크롤 소스를 DB 테이블로 관리
**선택:** bot_sources 테이블에 플랫폼명, 시드 URL, 크롤 주기, 활성화 여부 저장
**대안:** 코드에 하드코딩, 환경변수, 설정 파일
**근거:** API로 동적 추가/제거 가능. 코드 배포 없이 새 시드 URL 추가. 크롤 상태(마지막 실행 시간, 수집 건수)도 같이 저장.

### D6: fuguebot 시스템 계정
**선택:** creators 테이블에 고정 UUID로 시드하는 시스템 계정
**대안:** creator_id nullable로 변경, 별도 bot_pins 테이블
**근거:** 기존 핀 모델(creator_id FK 필수)을 변경하지 않음. 시스템 계정이면 프로필 페이지에서 봇이 만든 핀 목록도 자연스럽게 조회 가능.

## Risks / Trade-offs

- [외부 플랫폼 구조 변경] → 플러그인이 깨질 수 있음. 크롤 실패율 모니터링 + 알림으로 대응
- [미디어 저장 비용] → S3 비용 증가. 이미지 용량 제한(10MB)과 크롤 빈도 조절로 관리
- [robots.txt 차단] → 일부 플랫폼이 크롤러를 차단할 수 있음. User-Agent 명시하고 rate limit 엄격히 준수
- [중복 콘텐츠] → 같은 작품이 여러 플랫폼에 있을 수 있음. MVP에서는 URL 기반 dedup만 (플���폼 간 중복은 허용)
- [핀 모델 의존성] → 핀 생성 로직이 변경되면 fuguebot도 수정 필요. 핀 생성을 서비스 레이어로 추상화하여 공유

## Open Questions

1. MVP 플러그인 2개: Pixiv + SoundCloud? Pixiv + YouTube? 사용자 결정 필요
2. 미디어 사이즈 제한: 이미지 10MB, 오디오 50MB, 비디오는 썸네일만? 결정 필요
3. 크롤 주기: 매시간? 매일? 플랫폼별로 다르게? 설정 가능하되 기본값 결정 필요
