# Tasks — fix-harvester-og-image-cache-cleanup

## 1. 저장소 계층 (키 존중 업로드 + URL 기반 삭제)

- [x] 1.1 `storage.Client`에 `KeyFromURL(rawURL string) (string, bool)` 추가 — public URL prefix 일치 시 key 반환, 불일치 시 false (D1)
- [x] 1.2 `storage.Client`에 `UploadWithKey(ctx, key, contentType, size, body)` 추가 — 기존 `Upload`와 동일 검증, 키는 호출자 값 사용 (D6)
- [x] 1.3 `bot.StorageAdapter.Upload`를 `UploadWithKey` 위임으로 전환 — 캐시 객체가 실제로 `images/` 네임스페이스에 저장되도록 (D6)
- [x] 1.4 `bot.Storage` 인터페이스에 `DeleteByURL(ctx, url) error` 추가, `StorageAdapter` 구현 — 자사 URL이 아니거나 key가 이미지 캐시 네임스페이스 밖이면 삭제 없이 성공, 캐시 네임스페이스 key만 `Client.Delete` 위임. 경계 판정은 `imageCacheKeyPrefix + "/"` 기준 (D1)
- [x] 1.5 `bot.MockStorage`에 `DeleteByURLFunc` + 호출 기록 추가, `NewMockStorage` 기본 no-op 제공 (기존 테스트 무수정 통과 확인)

## 2. upsert 쿼리 확장 (교체 이전 참조 반환)

- [x] 2.1 `pins.sql`의 `UpsertBotPinByURL`에 upsert 전 스냅샷 CTE를 추가해 `prev_og_image`를 함께 RETURNING (D2)
- [x] 2.2 `sqlc generate` 실행 및 기존 호출부 빌드 확인

## 3. ProcessDocument 정리 경로

- [x] 3.0 og_data 직렬화(`MarshalOGData`)를 캐시 저장 앞으로 이동 — 캐시 성공 후 upsert 도달 전 실패 창 제거 (D3)
- [x] 3.1 upsert 실패 시 보상 삭제: 이번 처리에서 캐시 저장이 성공했을 때만 새 객체 URL로 `DeleteByURL` 호출, 실패 semantics 불변 (D3)
- [x] 3.2 upsert 성공 시 교체 정리: `prev_og_image`가 존재하고 새 값과 다르면 `DeleteByURL(prev)` 호출 (자사·네임스페이스 판정은 어댑터) (D3)
- [x] 3.3 삭제 실패는 대상 URL과 사유가 식별 가능한 로그만 남기고 반환값에 영향 없음 확인 (D4)

## 4. 단위 테스트 (delta spec 시나리오 커버)

- [x] 4.1 upsert 실패 + 캐시 성공 → 새 객체 보상 삭제 호출됨 / upsert 실패 + fallback → 삭제 호출 없음
- [x] 4.2 재수집 교체 → prev 삭제 호출됨: (a) 새 캐시 객체로 교체, (b) 원본 URL fallback으로 교체, (c) 참조 부재(NULL)로 교체
- [x] 4.3 StorageAdapter 판정: prev 외부 URL → 삭제 미수행 / prev 자사 URL이지만 캐시 네임스페이스 밖(예: 사용자 미디어 key, 구분자 없는 `imagesfoo/...` key) → 삭제 미수행 / 캐시 네임스페이스 key → 삭제 수행
- [x] 4.4 파이프라인 판정: prev == new → 삭제 호출 없음 / 신규 insert(prev NULL) → 삭제 호출 없음
- [x] 4.5 삭제 실패 시 ProcessDocument 반환값(성공/실패, created, pinID) 불변 + 로그에 대상 URL과 사유 포함 확인
- [x] 4.6 `KeyFromURL` 단위 테스트 (prefix 일치/불일치/경계) 및 `UploadWithKey`가 호출자 키로 저장·검증 유지 확인
- [x] 4.7 og_data 직렬화 실패 시 캐시 업로드(`Storage.Upload`) 호출이 발생하지 않음(고아 객체 생성 창 없음) — task 3.0 순서 회귀 방지

## 5. 검증

- [x] 5.1 `go build ./...` 및 bot·storage 패키지 테스트 전체 통과
- [x] 5.2 실환경 QA: MinIO + 로컬 파이프라인으로 (a) 캐시 객체가 `images/` 네임스페이스에 저장되는지, (b) 재수집 시 이전 캐시 객체가 버킷에서 제거되는지 관찰
