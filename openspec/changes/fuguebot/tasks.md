## 1. 기반 설정

- [ ] 1.1 `go get github.com/gocolly/colly/v2` 의존성 추가
- [ ] 1.2 `apps/api/cmd/bot/main.go` 엔트리포인트 생성
- [ ] 1.3 `internal/bot/` 패키지 구조 생성 (engine.go, source.go, downloader.go, dedup.go)
- [ ] 1.4 DB 시드에 fuguebot 시스템 계정 추가 (고정 UUID)
- [ ] 1.5 bot_sources 테이블 마이그레이션 작성 (id, name, platform, seed_urls, interval, is_active, last_crawled_at, stats)

## 2. Source 인터페이스 + 크롤 엔진

- [ ] 2.1 Source interface 정의 (Name, SeedURLs, Configure, Extract)
- [ ] 2.2 RawItem struct 정의 (Title, Description, MediaURL, SourceURL, Tags)
- [ ] 2.3 Colly 기반 크롤 엔진 구현 (robots.txt, rate limit, User-Agent: Fuguebot/1.0)
- [ ] 2.4 등록된 소스 순회 로직 (bot_sources 테이블에서 활성 소스 로딩)
- [ ] 2.5 크롤 엔진 단위 테스트

## 3. 미디어 다운로드 + S3 업로드

- [ ] 3.1 미디어 다운로더 구현 (HTTP GET → 바이트 스트림)
- [ ] 3.2 MIME 타입 검증 (이미지: JPEG/PNG/WebP/GIF, 오디오: MP3/OGG/WAV, 비디오: MP4/WebM)
- [ ] 3.3 파일 크기 제한 적용 (설정 가능, 기본값 결정 필요)
- [ ] 3.4 S3 미디어 버킷 업로드 로직 (AWS SDK v2)
- [ ] 3.5 다운로드 실패 시 skip + 에러 로깅
- [ ] 3.6 미디어 다운로더 단위 테스트

## 4. 핀 생성 파이프라인

- [ ] 4.1 URL 중복 체크 (pins 테이블에서 source_url 조회)
- [ ] 4.2 자동 태깅 로직 연동 (기존 auto-suggest 재사용)
- [ ] 4.3 핀 생성 (creator_id = fuguebot, media_key, source_url, title, description, tags)
- [ ] 4.4 파이프라인 통합 테스트 (Source → Download → Dedup → Store)

## 5. MVP 플러그인

- [ ] 5.1 첫 번째 플러그인 구현 (이미지 중심 플랫폼)
- [ ] 5.2 두 번째 플러그인 구현 (음악/영상 중심 플랫폼)
- [ ] 5.3 각 플러그인 테스트 (HTML fixture 기반)

## 6. 크롤 통계 + 관리 API

- [ ] 6.1 크롤 통계 이벤트를 Firehose로 전송 (기존 이벤트 파이프라인 재사용)
- [ ] 6.2 GET /api/admin/bot/status — 크롤 상태 조회 핸들러
- [ ] 6.3 GET /api/admin/bot/sources — 소스 목록 조회 핸들러
- [ ] 6.4 POST /api/admin/bot/sources — 소스 추가 핸들러
- [ ] 6.5 DELETE /api/admin/bot/sources/:id — 소스 삭제 핸들러
- [ ] 6.6 관리 API 테스트

## 7. 통합 + 배포 준비

- [ ] 7.1 cmd/bot/main.go에서 전체 파이프라인 연결 (엔진 → 소스 → 다운로드 → 저장 → 통계)
- [ ] 7.2 설정: 환경변수로 S3 버킷, DB DSN, Firehose 스트림명 주입
- [ ] 7.3 Dockerfile 작성 (apps/api/Dockerfile.bot)
- [ ] 7.4 E2E 테스트 (로컬에서 전체 크롤 실행)
