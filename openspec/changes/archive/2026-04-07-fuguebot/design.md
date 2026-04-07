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

D11에서 재정의 — 아래 D11 참조.

### D4: 미디어 다운로드 → S3 저장
**선택:** fuguebot이 직접 HTTP GET → 임시 버퍼 → `storage.Client.Upload()` 재사용
**대안:** 미디어 URL만 저장하고 프론트에서 직접 로드
**근거:** 외부 미디어 URL은 언제든 깨질 수 있음 (hotlink 방지, 삭제). 핀 모델이 미디어 필수이므로 자체 저장 필수. 기존 `internal/storage/` 패키지를 재사용하되, HTTP 응답의 Content-Length가 없는 경우 메모리 버퍼링 후 size를 계산하여 Upload에 전달. 다운로드 타임아웃: 이미지 30초, 오디오 120초, 비디오 300초.

### D5: 크롤 소스를 DB 테이블로 관리
**선택:** bot_sources 테이블에 플랫폼명, 시드 URL, 크롤 주기, 활성화 여부 저장
**대안:** 코드에 하드코딩, 환경변수, 설정 파일
**근거:** API로 동적 추가/제거 가능. 코드 배포 없이 새 시드 URL 추가. 크롤 상태(마지막 실행 시간, 수집 건수)도 같이 저장.

```sql
CREATE TABLE bot_sources (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(100) NOT NULL,
    platform VARCHAR(50) NOT NULL,
    seed_urls TEXT[] NOT NULL,
    interval_hours INT NOT NULL DEFAULT 24,
    enabled BOOLEAN NOT NULL DEFAULT true,
    last_crawled_at TIMESTAMPTZ,
    stats JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

### D6: fuguebot 시스템 계정
**선택:** creators 테이블에 고정 UUID `00000000-0000-0000-0000-00000000f096`로 시드하는 시스템 계정
**대안:** creator_id nullable로 변경, 별도 bot_pins 테이블
**근거:** 기존 핀 모델(creator_id FK 필수)을 변경하지 않음. 시스템 계정이면 프로필 페이지에서 봇이 만든 핀 목록도 자연스럽게 조회 가능.

### D7: URL 중복 체크 범위 — 전체 pins 테이블
**선택:** 전체 pins 테이블에서 url 컬럼 기준 dedup (봇 + 유저 핀 모두 대상)
**근거:** 유저가 이미 핀한 콘텐츠를 봇이 중복 생성하면 피드에 같은 작품이 두 번 나타남. 유저 핀을 우선시하고 봇은 보충 역할. pins.url에 부분 인덱스(`WHERE url IS NOT NULL`) 추가.

### D8: 크롤 동시성 제어 — K8s concurrencyPolicy: Forbid
**선택:** K8s CronJob `concurrencyPolicy: Forbid` 설정으로 이전 실행이 완료될 때까지 다음 실행 차단
**근거:** 동시 실행 시 같은 콘텐츠를 중복 다운로드/저장하는 레이스 컨디션 발생. Forbid가 가장 단순하고 안전.

### D9: 태그 자동 추출 실패 처리 — skip
**선택:** 매칭 태그가 0개인 항목은 핀 생성을 건너뜀
**근거:** 기존 핀 모델이 태그 1개 이상 필수. 태그 없는 핀은 추천/탐색 불가하므로 생성 가치 없음.

### D10: 관리 API 인증 — 환경변수 API key
**선택:** `X-Admin-Key` 헤더와 환경변수 `ADMIN_API_KEY`를 비교하는 미들웨어
**근거:** MVP에서는 admin role/RBAC 미구현. API key가 가장 단순. 프로덕션에서는 K8s 내부 네트워크 + API key 이중 보호.

### D11: Source 인터페이스 추상화 — Crawl 메서드
**선택:** API 기반 소스도 지원하기 위해 Source 인터페이스를 `Crawl(ctx) ([]RawItem, error)` 단일 메서드로 변경
**근거:** Unsplash는 REST API 기반이므로 Colly HTMLElement를 전제하는 인터페이스가 맞지 않음. Crawl 메서드 하나로 HTML 크롤링이든 API 호출이든 내부 구현을 자유롭게.

```go
type Source interface {
    Name() string
    Crawl(ctx context.Context) ([]RawItem, error)
}
```

## Risks / Trade-offs

- [외부 플랫폼 구조 변경] → 플러그인이 깨질 수 있음. 크롤 실패율 모니터링 + 알림으로 대응
- [미디어 저장 비용] → S3 비용 증가. 이미지 용량 제한(10MB)과 크롤 빈도 조절로 관리
- [robots.txt 차단] → 일부 플랫폼이 크롤러를 차단할 수 있음. User-Agent 명시하고 rate limit 엄격히 준수
- [중복 콘텐츠] → 같은 작품이 여러 플랫폼에 있을 수 있음. MVP에서는 URL 기반 dedup만 (플���폼 간 중복은 허용)
- [핀 모델 의존성] → 핀 생성 로직이 변경되면 fuguebot도 수정 필요. 핀 생성을 서비스 레이어로 추상화하여 공유

## Resolved Questions

1. MVP 플러그인 2개: **Unsplash (이미지, 공개 API) + Free Music Archive (음악, CC 라이선스)**. 저작권 리스크 최소화를 위해 공개/CC 라이선스 플랫폼 선택.
2. 미디어 사이즈 제한: 기존 `internal/storage/` 패키지의 maxBytes 재사용 (이미지 10MB, 오디오 50MB, 비디오 100MB).
3. 크롤 주기: 기본 24시간 (일일 1회). bot_sources 테이블에서 소스별 interval 설정 가능.
4. 크롤 통계: Kinesis Firehose 대신 **bot_sources 테이블의 stats JSON 필드에 직접 기록**. 별도 이벤트 파이프라인은 오버엔지니어링. 추후 필요 시 전환.
