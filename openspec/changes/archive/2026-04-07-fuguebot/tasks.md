## 1. 기반 설정

- [x] 1.1 `go get github.com/gocolly/colly/v2` 의존성 추가
- [x] 1.2 `apps/api/cmd/bot/main.go` 엔트리포인트 생성
- [x] 1.3 `internal/bot/` 패키지 구조 생성 (engine.go, source.go, downloader.go, dedup.go)
- [x] 1.4 DB 시드에 fuguebot 시스템 계정 추가 (UUID: `00000000-0000-0000-0000-00000000f096`)
- [x] 1.5 bot_sources 테이블 마이그레이션 작성 + down 파일
- [x] 1.6 pins.url 부분 인덱스 마이그레이션 추가 (`CREATE INDEX idx_pins_url ON pins(url) WHERE url IS NOT NULL`)

## 2. Source 인터페이스 + 크롤 엔진

- [x] 2.1 Source interface 정의 (`Name() string`, `Crawl(ctx) ([]RawItem, error)`)
- [x] 2.2 RawItem struct 정의 (Title, Description, MediaURL, MediaType, SourceURL)
- [x] 2.3 크롤 엔진 구현 (등록된 Source 순회 실행, 에러 수집)
- [x] 2.4 bot_sources 테이블에서 활성 소스 로딩 쿼리 + sqlc 생성
- [x] 2.5 크롤 엔진 단위 테스트

## 3. 미디어 다운로드 + S3 업로드

- [x] 3.1 미디어 다운로더 구현 (HTTP GET → 메모리 버퍼 → size 계산, 타임아웃: 이미지 30초/오디오 120초/비디오 300초)
- [x] 3.2 MIME 타입 검증 (기존 `internal/storage/` 패키지의 allowedMIME 재사용)
- [x] 3.3 파일 크기 제한 적용 (기존 `internal/storage/` 패키지의 maxBytes 재사용)
- [x] 3.4 S3 업로드 로직 (기존 `internal/storage/` 패키지의 Upload 메서드 재사용)
- [x] 3.5 다운로드 실패 시 skip + 에러 로깅
- [x] 3.6 미디어 다운로더 단위 테스트

## 4. 핀 생성 파이프라인

- [x] 4.1 URL 중복 체크 쿼리 추가 (전체 pins 테이블에서 url 일치 확인) + sqlc 생성
- [x] 4.2 자동 태깅 로직 (제목/설명 텍스트에서 tags 테이블의 name과 매칭 → UUID 조회 → pin_tags에 INSERT, 매칭 0개 시 항목 skip)
- [x] 4.3 핀 생성 (creator_id = fuguebot UUID, media_url, media_type, url, title, description + pin_tags 연결)
- [x] 4.4 파이프라인 통합 테스트 (Source → Download → Dedup → TagMatch → Store)

## 5. MVP 플러그인

- [x] 5.1 `internal/bot/sources/unsplash.go` — Unsplash 플러그인 (REST API 기반, 이미지 중심)
- [x] 5.2 `internal/bot/sources/fma.go` — Free Music Archive 플러그인 (Colly HTML 크롤링, 음악 중심)
- [x] 5.3 각 플러그인 테스트 (HTTP fixture/mock 기반)

## 6. 크롤 통계 + 관리 API

- [x] 6.1 크롤 통계를 bot_sources 테이블 stats 필드에 기록 (last_crawled_at, 수집/skip/실패 건수)
- [x] 6.2 관리 API 인증 미들웨어 (X-Admin-Key 헤더 + ADMIN_API_KEY 환경변수 비교)
- [x] 6.3 GET /api/admin/bot/status — 크롤 상태 조회 핸들러
- [x] 6.4 GET /api/admin/bot/sources — 소스 목록 조회 핸들러
- [x] 6.5 POST /api/admin/bot/sources — 소스 추가 핸들러
- [x] 6.6 PATCH /api/admin/bot/sources/:id — 소스 활성/비활성 토글 핸들러
- [x] 6.7 DELETE /api/admin/bot/sources/:id — 소스 삭제 핸들러
- [x] 6.8 관리 API 테스트

## 7. 통합 + 배포 준비

- [x] 7.1 cmd/bot/main.go에서 전체 파이프라인 연결 (엔진 → 소스 → 다운로드 → 저장 → 통계)
- [x] 7.2 설정: 환경변수로 S3 버킷, DB DSN, ADMIN_API_KEY 주입
- [x] 7.3 Dockerfile 작성 (apps/api/Dockerfile.bot)
- [x] 7.4 K8s CronJob manifest 작성 (concurrencyPolicy: Forbid)
- [x] 7.5 E2E 테스트 (로컬에서 전체 크롤 실행)
