# Proposal: HarvestPipeline title rune-safe truncate

## Why

`apps/api/internal/bot/harvester_consumer.go:380-382`는 Pin upsert 직전 `doc.BodyText = truncateRunes(doc.BodyText, 500)`를 호출해 `pins.description`의 `VARCHAR(500)` cap에 맞춰 본문을 사전 절단한다. 그러나 같은 PinDocument 안의 `doc.Title`은 어떤 length cap도 거치지 않고 `apps/api/internal/bot/harvest_pipeline.go:293`의 `UpsertBotPinByURL{Title: doc.Title}`로 그대로 들어간다. `pins.title` 컬럼은 `apps/api/db/migrations/000003_create_works.up.sql:5`에서 `VARCHAR(200) NOT NULL`로 정의되었고 이후 ALTER가 없으므로 production 스키마와 일치한다.

결과: 외부 사이트의 `<title>`/`<h1>`/`og:title`이 201 rune 이상이면 PostgreSQL이 `value too long for type character varying(200)`로 INSERT를 거부 → `createPins`가 에러를 반환 → `harvester_consumer`가 `scheduler.ErrorNetwork`로 retry 큐에 다시 넣음 → title은 변하지 않으니 같은 에러로 5회 반복 실패 → `harvest_error_count`가 5에 도달해 partial index에서 제외됨. 그동안 snapshot fetch + extract + DB INSERT 시도를 5회 반복하는 비용 누수가 일어나고, 결과적으로 정상 컨텐츠의 Pin이 영구 누락된다.

이는 spec 위반이기도 하다. `openspec/specs/harvester/spec.md:190`의 Pin upsert SHALL "INSERT ... ON CONFLICT DO UPDATE를 통해 기존 Pin의 title, description, og_image, og_data를 새 값으로 갱신한다"가 title 길이에 따라 비결정적으로 깨진다. 또한 같은 spec L238-240이 description에 대해서만 "500 rune 이내로 잘라 저장한다 (UTF-8 rune-safe)"를 명시하고 title에 대해서는 동일 계약을 명시하지 않은 비대칭 결함이다.

## What Changes

1. **harvester consumer 단일 지점에 title 사전 절단 추가** — `apps/api/internal/bot/harvester_consumer.go`의 `processOne`에서 `doc.BodyText = truncateRunes(doc.BodyText, 500)` 바로 뒤에 `doc.Title = truncateRunes(doc.Title, 200)` 한 줄을 대칭으로 추가한다. ScriptAdapter 경로도 consumer를 통과하므로 단일 지점으로 두 경로를 모두 커버한다.

2. **pin_document.go doc comment에 Title truncate 책임 명시** — `apps/api/internal/bot/pin_document.go:36-38`의 PinDocument doc-comment에 "Title is raw text BEFORE the title-length cut; the Harvester is responsible for the rune-safe truncation when persisting." 한 줄을 추가해 BodyText와 대칭으로 책임 위치를 문서화한다.

3. **truncateRunes 함수 doc-comment 업데이트** — `apps/api/internal/bot/harvester_consumer.go:490-492`의 함수 doc-comment를 "Used to fit PinDocument.BodyText into the pins.description column's 500-rune limit before persistence."에서 "Used to fit PinDocument.BodyText / PinDocument.Title into the pins.description / pins.title column rune limits before persistence."로 확장한다.

4. **회귀 방지 테스트** — `apps/api/internal/bot/harvester_consumer_test.go`(또는 새 `harvester_consumer_title_truncate_test.go`)에 다음 케이스 추가:
   - 201자 ASCII title 입력 → 200자 truncate 확인
   - 201자 한국어 title (멀티바이트) 입력 → rune 경계로 200 rune truncate 확인
   - 100자 정상 title 입력 → 무손실 통과 확인
   - 빈 title 입력 → 빈 문자열 유지 확인

5. **harvester spec에 ADDED Scenario** — `openspec/specs/harvester/spec.md`의 "PinDocument 부가 필드 og_data 저장 정책" 섹션(또는 가장 가까운 PinDocument persistence 섹션)에 "title은 pins.title 컬럼에 200 rune 이내로 잘라 저장한다 (UTF-8 rune-safe, multi-byte 문자를 바이트 경계에서 절단하지 않는다)" Scenario를 ADDED 한다.

## Why Now / Why Self-Contained

- **Why Now**: Discovery 모드에서 발견된 정합성 결함. spec(L238-240)이 description의 동일 패턴을 이미 enforce하므로 title의 비대칭은 명백히 누락된 SHALL이다. fix가 한 줄이므로 미루는 비용보다 처리 비용이 현저히 낮다.
- **Why Self-Contained**: 변경 범위가 (a) consumer 한 줄 추가, (b) 두 doc-comment 확장, (c) 테스트 1개 파일, (d) spec Scenario 1개 ADDED로 전부 한 changeset 안에 닫힌다. DB 마이그레이션, 다른 패키지 변경, infra 영향 없음.

## Scope

- 변경 파일: `apps/api/internal/bot/harvester_consumer.go`(1 line addition + 1 comment update), `apps/api/internal/bot/pin_document.go`(1 line comment addition), `apps/api/internal/bot/harvester_consumer_title_truncate_test.go`(신규 테스트 파일), `openspec/specs/harvester/spec.md`(1 ADDED Scenario).
- 변경 외 파일: HarvestPipeline (`harvest_pipeline.go`)는 손대지 않는다. `pickMediaForPin`, `ProcessDocument`의 다른 로직은 무영향.
- `cmd/bot/main.go`, `cmd/server/main.go` 부트스트랩 변경 없음.

## Out of Scope

- `pins.title` 컬럼 길이 확장(예: VARCHAR(500)) 마이그레이션 — DB 영구 변경은 별개 change. 현 spec과 ERD는 200 rune cap을 합의된 contract로 둔다.
- ScriptAdapter 자체 변경 — primary.Title 길이 검증을 ScriptAdapter 안에서 강제하는 안. consumer 한 지점에서 충분히 커버되므로 보류.
- Description의 truncate 로직 변경 — 이미 잘 작동하는 부분 손대지 않는다.

## Rollback

`harvester_consumer.go`의 추가된 한 줄과 spec Scenario, doc-comment 한 줄을 revert. 테스트 파일 삭제. DB나 데이터 변환이 없으므로 즉시 가역.

## QA Plan (실 환경)

1. `docker-compose up -d`로 api+postgres 기동.
2. `python3 -m http.server 8090`로 호스트에 다음 HTML serve:
   ```html
   <html><head><title>${201자 또는 한국어 100+ rune title}</title>
   <meta property="og:image" content="..."></head>
   <body><article><p>${본문 200자 이상}</p></article></body></html>
   ```
3. `psql`로 `pioneer_frontier`에 위 URL을 enqueue (또는 직접 Pioneer 1회 실행).
4. Pioneer 1회 실행 → snapshot이 `harvester_frontier`로 fanout되는지 확인.
5. Harvester 1회 실행 → 로그에서 `pq: value too long for type character varying(200)` **부재** 확인(fix 적용 후), Pin이 정상 생성되는지 확인.
6. `psql -c "SELECT length(title), title FROM pins WHERE url='http://localhost:8090/long.html';"` → 길이 ≤ 200, title의 마지막 글자가 멀티바이트 한국어인 경우 글자 경계에서 깨끗하게 잘렸는지 확인.
7. 회귀: 정상(50 rune) title 페이지로 동일 절차 → Pin 정상 생성, title 입력값과 동일.
8. ScriptAdapter 경로: 짧은 site script가 등록된 도메인 1개로 동일 절차 1회 수행 → 동일하게 truncate 적용 확인.

## Threat Model / Failure Mode

- **이전 (fix 없음)**: 201 rune title → INSERT 실패 → 5회 retry → 5x snapshot/extract 비용 누수 → 영구 누락. 외부 사이트의 정상 컨텐츠 일부 잠재적 누락. 악의적 도메인의 amplification(1 long-title 페이지 × 5 retry).
- **이후 (fix 적용)**: 201 rune title → 200 rune truncate → INSERT 성공 → Pin 정상 생성 + 1회 fetch로 완료. spec의 Pin upsert SHALL이 결정적으로 충족됨.
