## 1. DB 마이그레이션

- [ ] 1.1 BotCreatorID 운영 정책 정리 (환경변수 vs 코드 상수) 후 단일 ID 결정
- [ ] 1.2 기존 봇 Pin 중 같은 URL의 중복 행을 dedup하는 일회성 SQL 스크립트 작성 (가장 최근 created_at만 유지)
- [ ] 1.3 마이그레이션 추가: `CREATE UNIQUE INDEX CONCURRENTLY pins_url_bot_unique ON pins(url) WHERE creator_id = '<BotCreatorID>'`
- [ ] 1.4 down 마이그레이션: `DROP INDEX CONCURRENTLY IF EXISTS pins_url_bot_unique`
- [ ] 1.5 sqlc 쿼리 추가: `UpsertBotPinByURL` (`INSERT ... ON CONFLICT (url) WHERE creator_id = $bot_id DO UPDATE SET title=..., description=..., og_image=..., og_data=...`)

## 2. PinDocument 도메인 타입

- [ ] 2.1 `apps/api/internal/bot/pin_document.go` 신규 작성: `PinDocument` struct (title, body_text, canonical_url, thumbnail_url, media_candidates, lang, author, published_at, og_data)
- [ ] 2.2 `MediaCandidate` struct 정의 (type, url, width, height)
- [ ] 2.3 `og_data` 직렬화 헬퍼: classifier 결과/extractor 식별자/source URL 보존 키 구조 정의
- [ ] 2.4 unit test: PinDocument 직렬화/역직렬화 라운드트립

## 3. Generic HTML→Pin extractor

- [ ] 3.1 `apps/api/internal/bot/extractor.go` 신규 작성: `GenericExtractor.Extract(html, fetchURL) (PinDocument, error)`
- [ ] 3.2 OG/Twitter Card 메타 파서 구현 (title, image, url, description, locale, article:published_time, article:author)
- [ ] 3.3 JSON-LD `schema.org` 파서 구현 (Article, CreativeWork: headline, articleBody, image, author, datePublished)
- [ ] 3.4 `<article>` / 최대 텍스트 밀도 블록 추출 로직
- [ ] 3.5 `<title>` / `<h1>` / `<link rel=canonical>` / `<html lang>` / `<time datetime>` 태그 파서
- [ ] 3.6 본문 내 `<img>`/`<video>`/`<audio>`/`<source>` 수집 + 절대 URL 변환
- [ ] 3.7 cross-domain canonical 무시 정책 적용
- [ ] 3.8 fallback 체인 우선순위 적용 (title/body_text/canonical/thumbnail 각각)
- [ ] 3.9 unit test: OG만 있는 페이지, JSON-LD만 있는 페이지, `<article>`만 있는 페이지, 셋 다 없는 페이지, cross-domain canonical 케이스
- [ ] 3.10 PinDocument는 항상 nil이 아닌 객체를 반환함을 보장하는 테스트

## 4. Content classifier

- [ ] 4.1 `apps/api/internal/bot/classifier.go` 신규 작성: `Classifier.Classify(doc PinDocument, nodeType string) (pinnable bool, reason string)`
- [ ] 4.2 사유 우선순위 (`listing` > `empty_body` > `no_primary_media` > `low_text_link_ratio`) 적용
- [ ] 4.3 `listing` 판정: nodeType == "list" OR outgoing-link/word 비율
- [ ] 4.4 `empty_body` 판정: body_text < 임계값 (기본 200자, 설정 가능)
- [ ] 4.5 `no_primary_media` 판정: thumbnail 없음 AND media_candidates 비어 있음 AND body_text 임계값 미만
- [ ] 4.6 `low_text_link_ratio` 판정: body_text 길이 / outgoing 링크 수 < 임계값
- [ ] 4.7 classifier 결과를 `og_data.classifier` 키에 보존
- [ ] 4.8 unit test: 사유 우선순위, 정상 페이지, 각 사유별 경계 케이스

## 5. PerSiteAdapter / AdapterRegistry

- [ ] 5.1 `apps/api/internal/bot/adapter.go` 신규 작성: `PerSiteAdapter` 인터페이스 (`Domain()`, `Extract(ctx, html, fetchURL)`)
- [ ] 5.2 `AdapterRegistry` 인터페이스 + 인메모리 구현 (`Resolve(domain)`, `Register(adapter)`)
- [ ] 5.3 도메인 매칭 정책 (정확 일치 + 와일드카드 서브도메인) 정의
- [ ] 5.4 `og_data.extractor` 식별자 채우기 헬퍼 (`generic`, `script:<site_id>`, `<adapter_name>`)
- [ ] 5.5 unit test: register/resolve 동작, 미등록 도메인 fallback

## 6. ScriptAdapter (기존 GojaExecutor 래핑)

- [ ] 6.1 `apps/api/internal/bot/script_adapter.go` 신규 작성: `ScriptAdapter` struct가 `PerSiteAdapter` 구현
- [ ] 6.2 기존 `GojaExecutor`를 의존성 주입으로 ScriptAdapter 내부에 보유
- [ ] 6.3 `Extract`에서 (site_id, node_type) 스크립트 로드 → 실행 → RawItem 배열 수신
- [ ] 6.4 N개 RawItem → PinDocument 1건 축약: 첫 번째를 정본 메타로, 나머지를 `og_data.media_candidates`에 추가
- [ ] 6.5 빈 결과(0건) 또는 실행 실패 시 에러 반환 → Harvester가 generic으로 fallback
- [ ] 6.6 부트스트랩 시 DB의 (site_id, node_type) 스크립트가 있는 사이트의 도메인을 AdapterRegistry에 등록
- [ ] 6.7 unit test: N개 RawItem 축약, 빈 결과 처리, 실행 실패 처리

## 7. Harvester 결합

- [ ] 7.1 `apps/api/internal/bot/harvester.go`의 `executeNode`를 PinDocument 반환으로 변경
- [ ] 7.2 처리 순서 적용: `adapter, ok = registry.Resolve(domain) → adapter.Extract OR generic.Extract → classifier.Classify → upsert OR mark harvested_at`
- [ ] 7.3 어댑터 실패 시 generic fallback 경로 구현 (AdapterFallback 통계 카운트 증가)
- [ ] 7.4 `og_data.source` 보존 (canonical_url과 다를 때 원본 fetch URL)
- [ ] 7.5 `media_candidates` 길이 상한(기본 50) 적용
- [ ] 7.6 `body_text`는 `pins.description` (500자 제한)에 잘라 넣고 og_data에는 중복 저장하지 않음
- [ ] 7.7 `media_url` NOT NULL 제약 충족: thumbnail_url 또는 첫 media_candidates URL 사용
- [ ] 7.8 통계 재정의: `PinsCreated`, `Deduped`, `Skipped`, `Failed`, `AdapterFallback` 5개 카테고리
- [ ] 7.9 노드 1개 = 통계 1건 보장 (ScriptAdapter N개 RawItem이어도 노드 단위 1건)
- [ ] 7.10 pinnable=false 노드는 frontier row의 `harvested_at`만 마킹 (Pin 생성/update 없음)

## 8. Harvest pipeline 정리

- [ ] 8.1 `apps/api/internal/bot/harvest_pipeline.go`를 PinDocument 기반 upsert로 갱신
- [ ] 8.2 기존 RawItem 흐름은 ScriptAdapter 내부로만 한정 (외부 인터페이스에서 제거)
- [ ] 8.3 `UpsertBotPinByURL` 호출 결과를 (created vs updated)로 구분하여 PinsCreated/Deduped 통계에 매핑
- [ ] 8.4 동시 upsert race를 가정한 통합 테스트 (두 워커 동시 호출 → 정확히 한 행 보장)

## 9. 설정 / Feature flag

- [ ] 9.1 `HARVESTER_DEFAULT_EXTRACTOR=generic|script` 환경변수 도입 (기본 `generic`)
- [ ] 9.2 임계값 설정 노출: `body_text` 최소 길이, `low_text_link_ratio` 임계, `media_candidates` 상한
- [ ] 9.3 BotCreatorID 설정 노출 (env or config)

## 10. 테스트 / 검증

- [ ] 10.1 generic extractor 시나리오 unit test (specs/harvester의 모든 시나리오 매핑)
- [ ] 10.2 classifier 시나리오 unit test (사유 우선순위 포함)
- [ ] 10.3 ScriptAdapter 시나리오 unit test (N→1 축약, fallback)
- [ ] 10.4 canonical-URL upsert 통합 테스트 (insert/update/race/일반 사용자 Pin과의 공존)
- [ ] 10.5 Harvester 통계 5-카테고리 통합 테스트
- [ ] 10.6 cross-domain canonical 무시 회귀 테스트
- [ ] 10.7 `media_url` NOT NULL 위반이 발생하지 않음을 확인하는 회귀 테스트
- [ ] 10.8 기존 `goja_executor_test.go` / `harvest_pipeline_test.go` 가 ScriptAdapter 경계 안에서 통과하도록 보강

## 11. 문서화 / Spec sync

- [ ] 11.1 `apps/api/internal/bot/README.md`에 generic extractor + adapter registry + classifier 흐름 추가
- [ ] 11.2 `docs/erd.md`에 partial unique index `pins_url_bot_unique` 와 og_data 키 구조 노트 추가
- [ ] 11.3 `openspec validate harvester-pin-document` 통과 확인
- [ ] 11.4 변경 archive 시 `openspec/changes/archive/2026-04-15-perfect-harvester/` 와의 관계(이전 정본 → 본 변경으로 강등) 메모 남기기

## 12. 롤아웃

- [ ] 12.1 dev 환경에서 partial unique index 생성 + dedup 스크립트 실행
- [ ] 12.2 staging에서 `HARVESTER_DEFAULT_EXTRACTOR=generic` 활성화 후 노드 단위 통계 검증
- [ ] 12.3 ScriptAdapter가 등록된 도메인(예: pixiv)에서 N→1 축약이 검색 결과 품질을 떨어뜨리지 않는지 샘플 확인
- [ ] 12.4 prod 롤아웃 + 메트릭 (PinsCreated/Deduped/Skipped/Failed/AdapterFallback) 모니터링
