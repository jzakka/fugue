# Design — fix-harvester-og-image-cache-cleanup

## Context

`HarvestPipeline.ProcessDocument`(apps/api/internal/bot/harvest_pipeline.go)는 다음 순서로 동작한다:

1. `cacheImage`가 후보 이미지를 다운로드·검증하고 `buildImageCacheKey(normalized, contentType, nowUnix)` 키로 `p.storage.Upload`를 호출해 저장소 URL을 얻는다. 실패 시 원본 후보 URL로 fallback한다.
2. `UpsertBotPinByURL`이 canonical URL 키로 Pin row를 upsert하며 `og_image = EXCLUDED.og_image`로 덮어쓴다.

정리 경로가 전혀 없으므로: (a) 2단계 실패 시 1단계에서 올라간 객체가 고아로 남고, (b) 재수집 시 이전 og_image가 가리키던 객체가 미참조 상태로 잔존한다. 유일한 방어선은 90일 TTL(`DefaultImageCacheTTLDays`, 버킷 lifecycle로 집행).

관련 제약:

- `bot.Storage` 인터페이스는 `Upload`만 정의한다. 삭제 능력이 없다.
- 스택 하위 change(fix-pin-create-orphan-media, PR #4118)가 `storage.Client.Delete(ctx, key)`를 추가했다(멱등: 없는 키 삭제도 성공). 본 change 브랜치는 그 위에 스택되어 있어 사용 가능하다.
- `storage.Client.Upload`는 public URL(`pubURL + "/" + key`)을 반환한다. DB에는 URL만 저장되므로 이전 객체 삭제에는 URL→key 역변환이 필요하다.
- **`storage.Client.Upload`는 호출자가 넘긴 filename을 무시하고 `<mediatype>/<uuid>.<ext>` 키를 스스로 생성한다.** 따라서 `cacheImage`가 `images/<hash>/<ts>.<ext>` 키를 구성해 넘겨도, 실제 프로덕션 객체는 사용자 업로드 미디어와 같은 `image/` 네임스페이스에 저장되고 있다. 이는 기존 bot 스펙의 네임스페이스 분리 요구(`이미지 캐시 네임스페이스가 본문 미디어 네임스페이스와 분리된다`)가 구현에서 미집행 상태라는 뜻이며, 네임스페이스 기반의 안전한 삭제를 불가능하게 하므로 본 change에서 함께 바로잡아야 하는 전제다(D6).
- `UpsertBotPinByURL`은 upsert **후** row만 반환한다. 교체 이전 og_image 값은 현재 얻을 수 없다.

## Goals / Non-Goals

**Goals:**

- upsert 실패 시 그 시도에서 새로 업로드한 캐시 객체의 보상 삭제.
- upsert 성공으로 og_image 참조가 교체될 때 이전 자사 캐시 객체 삭제.
- 삭제는 best-effort: 실패해도 Pin 처리 결과에 영향 없음(로그만), TTL이 최종 방어선.
- 캐시 업로드가 실제로 이미지 캐시 네임스페이스(`images/`) 키로 저장되도록 기존 네임스페이스 분리 요구를 구현에서 집행한다 — 네임스페이스 한정 삭제의 안전성 전제(D6).

**Non-Goals:**

- 캐시 키 구성·no-overwrite 계약·TTL 값 변경.
- 저장소 전수 스캔 방식의 GC(참조 없는 객체 일괄 회수). 본 change는 처리 경로 내 정리만 다룬다.
- 과거에 이미 누적된 미참조 객체의 소급 정리(TTL 만료에 위임). 특히 D6 이전에 `image/` 네임스페이스로 잘못 저장된 레거시 캐시 객체는 본 change의 삭제 대상 밖이다(Risks 참조).
- 자사 저장소 밖 URL(원본 fallback)의 삭제 — 외부 리소스는 건드리지 않는다.
- 레거시 `Process()`(RawItem 경로)의 고아 정리. 동일한 "캐시 성공 후 CreatePin 실패" 패턴이 존재하지만, 이 경로는 테스트 외 프로덕션 호출자가 없다(프로덕션 경로는 harvester consumer → `ProcessDocument`).

## Decisions

### D1. bot Storage 추상화에 URL 기반 삭제를 추가한다

`bot.Storage` 인터페이스에 `DeleteByURL(ctx context.Context, url string) error`를 추가한다. 키가 아닌 **URL**을 받는 이유: 파이프라인이 보유한 식별자는 (i) Upload 반환값과 (ii) DB의 이전 og_image 값 — 둘 다 URL이다. URL→key 역변환은 저장소 구성(public URL prefix)을 아는 어댑터 계층의 책임으로 두는 것이 파이프라인을 저장소 세부에서 격리한다.

- `storage.Client`에 `KeyFromURL(rawURL string) (string, bool)` 헬퍼를 추가한다: `pubURL + "/"` prefix가 일치하면 나머지를 key로 반환, 아니면 `false`.
- `bot.StorageAdapter.DeleteByURL`: `KeyFromURL`로 자사 URL 여부를 판정하고, 이어서 key가 **이미지 캐시 네임스페이스 안에 있을 때만** `Client.Delete(ctx, key)`를 위임한다. 경계 판정은 `imageCacheKeyPrefix + "/"`(디렉터리 구분자 포함) prefix 기준이다 — 상수 원값 `"images"`를 구분자 없이 쓰면 `imagesfoo/...` 같은 키가 오매칭된다. 자사 URL이 아니거나 캐시 네임스페이스 밖의 자사 key(사용자 미디어 `image/`·`audio/`·`video/`, 본문 미디어, 스냅샷 등)는 **삭제하지 않고 성공 처리**한다. 외부 URL 배제와 "캐시 객체만 삭제"라는 스펙 요구를 모두 어댑터 한 곳에서 집행해, 파이프라인의 어떤 실수로도 캐시 외 객체가 삭제될 수 없게 한다.
- `bot.MockStorage`에 `DeleteByURLFunc`와 호출 기록을 추가해 테스트에서 삭제 호출 여부·인자를 검증한다.

대안: (a) `Upload`가 key도 반환하도록 시그니처 변경 — 보상 삭제에는 충분하지만 이전 객체 삭제(DB에는 URL만 있음)를 해결하지 못해 기각. (b) 파이프라인이 직접 URL 파싱 — 저장소 public URL 구성이 어댑터 밖으로 새므로 기각.

### D2. 교체 이전 og_image는 upsert 문 자체가 반환한다 (CTE)

`UpsertBotPinByURL` 쿼리에 upsert 전 스냅샷 CTE를 추가해 이전 og_image를 원자적으로 함께 반환한다:

```sql
WITH prev AS (
    SELECT og_image FROM pins
    WHERE url = $4 AND creator_id = $1
)
INSERT INTO pins (...) VALUES (...)
ON CONFLICT (url) WHERE creator_id = '...f096'  -- arbiter만 리터럴 강제(partial index 추론)
DO UPDATE SET ...
RETURNING *, (xmax = 0) AS inserted,
    (SELECT og_image FROM prev) AS prev_og_image;
```

CTE는 INSERT와 같은 스냅샷에서 평가되므로 `prev_og_image`는 이번 upsert가 덮어쓰기 **전**의 값이다. 신규 insert면 NULL. 별도 사전 SELECT 대비 왕복 1회 절약과 read-then-write 레이스(두 워커가 같은 canonical URL을 동시 처리) 축소 효과가 있다.

CTE의 creator 조건은 리터럴이 아니라 파라미터(`$1`)를 사용한다 — 리터럴 강제는 ON CONFLICT arbiter의 partial unique index 추론에만 필요한 제약이며, CTE에 리터럴을 쓰면 literal-sync 지점이 하나 늘어 bot creator ID 오버라이드 시 CTE만 갱신 누락되면 prev가 항상 NULL이 되어 교체 정리가 조용히 무력화되는 위험이 있다.

대안: upsert 전에 별도 `GetBotPinByURL` 조회 — 쿼리 추가 + 왕복 1회 + 레이스 창이 커져 기각.

### D3. 정리 판정 규칙

upsert **성공** 후:

- `prev_og_image`가 존재하고 새 og_image 값과 **다르면** → `DeleteByURL(prev)` 호출. 자사 URL 여부·캐시 네임스페이스 판정은 어댑터가 수행하므로(D1) 파이프라인은 값 비교만 한다.
- 값이 같으면(재수집인데 캐시 실패 fallback이 동일 원본 URL로 반복되는 경우 등) 삭제하지 않는다.
- 새 값이 NULL(캐시 비활성/썸네일 부재)이어도 prev가 자사 객체면 교체로 간주해 삭제한다 — row가 더 이상 참조하지 않는 객체이므로.

upsert **실패** 후:

- 이번 처리에서 `cacheImage`가 **성공**했을 때만(반환 오류 없음 = 새 객체가 실제로 업로드됨) 그 반환 URL로 `DeleteByURL` 보상 삭제를 호출한다. fallback(원본 URL)이었다면 업로드된 객체가 없으므로 호출하지 않는다.
- 보상 삭제 후에도 upsert 오류는 기존과 동일하게 반환한다(처리 실패 semantics 불변).

안전성 근거: 삭제는 이미지 캐시 네임스페이스(`images/`) 키에만 허용된다(D1). 크롤한 외부 페이지의 og:image가 우연히 자사 버킷의 사용자 미디어 URL(`image/` 등)인 경우에도, 그 key는 캐시 네임스페이스 밖이므로 어댑터가 삭제를 거부한다 — 사용자 미디어의 오삭제 경로는 구조적으로 없다. 단, 캐시 키는 후보 URL과 초 단위 시각에서 파생되고 canonical 페이지 URL은 포함하지 않으므로, 서로 다른 페이지가 같은 후보 URL(예: 사이트 전역 기본 og:image)을 같은 초에 캐시하면 키가 공유되어 복수 row가 한 객체를 참조할 수 있다 — 이때의 삭제 영향은 Risks에서 수용 처리한다.

구현 순서 조정: `ProcessDocument`의 og_data 직렬화(`MarshalOGData`)는 현재 캐시 저장 **이후**에 있어, 직렬화 실패 시 새 캐시 객체가 upsert에 도달하지 못한 채 무보상 고아가 되는 창이 있다. 직렬화를 캐시 저장 **앞**으로 옮겨 이 창을 제거한다 — "캐시 성공 이후의 실패"가 upsert 실패 하나로 수렴하므로 보상 삭제 규칙이 전 구간을 덮는다.

### D4. 삭제는 비차단 best-effort

모든 `DeleteByURL` 호출은 실패해도 `ProcessDocument`의 반환값(성공/실패, created 여부, pinID)에 영향을 주지 않는다. 실패는 `log.Printf`로 사유와 대상 URL을 남긴다. 근거: 기존 TTL lifecycle이 삭제 누락의 최종 방어선으로 이미 존재하므로, 삭제 실패로 Pin 파이프라인을 막는 것은 가용성 손해만 있다. 이는 기존 스펙의 "만료 처리는 Pin 생성을 막지 않는다"와 같은 원칙이다.

### D5. context 처리

보상 삭제는 upsert 실패 직후에 실행되는데, 실패 원인이 ctx 취소일 수 있다. 삭제 호출은 원 ctx를 그대로 사용한다(별도 detached context 도입 안 함). ctx 취소로 삭제가 실패해도 D4에 따라 로그만 남고, 객체는 TTL로 회수된다. detached context + 자체 타임아웃은 셧다운 지연과 복잡도를 늘려 기각.

### D6. 캐시 업로드는 호출자 키를 존중해야 한다 (전제 버그 수정)

`storage.Client`에 `UploadWithKey(ctx, key, contentType, size, body)`를 추가한다. 기존 `Upload`와 동일한 검증(MIME sniff·allowlist·크기)을 수행하되, 키를 자체 생성하지 않고 호출자가 넘긴 key로 저장한다. `bot.StorageAdapter.Upload`는 이 메서드로 위임을 전환한다.

근거: 현재 `storage.Client.Upload`는 filename을 무시하고 `<mediatype>/<uuid>.<ext>` 키를 생성하므로, `cacheImage`의 `images/<hash>/<ts>.<ext>` 키가 실제로는 반영되지 않고 캐시 객체가 사용자 미디어 네임스페이스(`image/`)에 저장된다. 이 상태에서는 (a) 기존 스펙의 네임스페이스 분리 요구가 미집행이고, (b) `images/` prefix를 대상으로 하는 TTL lifecycle이 캐시 객체에 적용되지 않으며, (c) D1의 네임스페이스 한정 삭제가 모든 실제 객체를 놓친다. 키를 존중해야 새 캐시 객체부터 분리·TTL·정리가 모두 유효해진다.

대안: `StorageAdapter`가 `S3Client()`로 raw `PutObject`를 직접 호출(스냅샷 스토어 방식) — storage 레이어의 MIME 위조 방지 검증을 우회하게 되어 기각. 기존 `Upload`의 키 생성 로직을 filename 존중으로 변경 — 사용자 업로드 경로(핀 미디어)의 키 정책까지 바뀌는 광범위 영향이라 기각.

부수효과: `StorageAdapter.Upload`는 레거시 `Process()`의 본문 미디어 업로드(`bot/<uuid>.<ext>` 키 전달)도 공유하므로, 전환 후 그 경로의 객체도 `<mediatype>/<uuid>` 대신 `bot/` 네임스페이스에 저장된다. 이는 원래 의도된 본문 미디어 네임스페이스와의 정렬이며(레거시 경로는 프로덕션 호출자 없음 — Non-Goals 참조), 의도된 변화로 수용한다.

## Risks / Trade-offs

- [upsert 성공 ↔ prev 삭제 사이 크래시] → 이전 객체가 잔존하지만 현재와 동일한 상태(누적 1건)이며 TTL로 회수된다. 트랜잭션적 정합은 목표가 아니다.
- [동시 재수집 레이스: 두 워커가 같은 canonical URL을 동시에 upsert] → READ COMMITTED에서 두 워커의 CTE 스냅샷이 모두 기존 값(객체 O)을 볼 수 있고, 그 경우 둘 다 prev=O를 받아 O를 중복 삭제한다(삭제 멱등이라 무해). ON CONFLICT 경합에서 진 쪽 워커가 올린 새 객체는 최종 row가 참조하지 않는데 아무도 삭제하지 않아 고아로 남고, TTL로 회수된다. 신규 canonical URL의 동시 insert 변형에서도 방향만 다를 뿐 결과 등급은 같다: 선행 inserter A의 객체를 후행 B의 DO UPDATE가 덮지만 B의 스냅샷에는 row가 없어 prev=NULL → A의 객체가 고아로 남고 TTL로 회수된다. 어느 변형에서도 최종 row가 참조하는 객체가 삭제되는 순서는 성립하지 않는다(각 워커는 자신이 row를 새 값으로 덮은 뒤 자신이 읽은 prev만 지우므로). 봇 재수집 스케줄 특성상 동시성 확률 자체도 극히 낮아 수용.
- [모호 실패: upsert가 오류를 반환했지만 DB에는 커밋된 경우(커밋 후 연결 단절 등)] → 보상 삭제가 현재 row가 실제로 참조하게 된 새 객체를 지워 og_image가 dangling할 수 있다. 스펙상 캐시 참조 해소 실패는 capability 실패가 아니며(소비자 UX가 허용), 다음 재수집에서 새 캐시로 복구된다. 발생 확률이 극히 낮고 결과가 허용 범위라 수용.
- [공유 키 삭제: 같은 후보 URL이 같은 초에 두 번 캐시되어 키가 공유된 경우] → 서로 다른 페이지가 같은 후보 URL(예: 사이트 전역 기본 og:image)을 공유하는 경우든, 같은 페이지가 같은 초에 재처리(큐 중복 전달·재시도)되는 경우든, 한 처리의 교체·보상 삭제가 다른 처리로 살아남은 row의 og_image 참조를 끊어 dangling이 될 수 있다(특히 같은 초 재처리 + 2차 upsert 실패 → 보상 삭제가 1차 성공 row의 키를 삭제). 캐시 키에 canonical URL·고유 성분을 포함하는 대안은 키 파생 계약(후보 URL + 시점 파생)을 바꾸는 스펙 변경이라 기각. dangling은 capability 실패가 아니고 다음 재수집으로 치유되며, "같은 후보 URL + 같은 초" 동시성은 드물어 수용.
- [fallback으로 기록된 자사 캐시 URL 삭제: 크롤한 외부 페이지의 og:image가 자사 이미지 캐시 객체 URL 자체인 경우] → 캐시 실패 fallback으로 그 `images/` URL이 og_image에 그대로 기록되고, 이후 교체 정리 시 어댑터가 캐시 네임스페이스 안이므로 삭제해 그 객체를 참조하던 다른 Pin이 dangling될 수 있다. 발생 조건(외부 페이지가 자사 캐시 URL을 og:image로 사용 + 캐시 실패 + 이후 교체)이 극히 이례적이고, 결과 등급이 공유 키 리스크와 동일(capability 실패 아님, 재수집 치유)해 수용.
- [D6 이전에 `image/` 네임스페이스로 저장된 레거시 캐시 객체] → 캐시 네임스페이스 밖이므로 본 change의 삭제 대상이 아니고, `image/`에는 lifecycle도 없어 잔존한다. 소급 정리는 Non-Goal이며, 필요 시 후속 change(bot Pin og_image 참조 대조 기반 일회성 정리)로 다룬다. 본 change 배포 후 신규 객체부터는 누적이 멈춘다.
- [MockStorage 시그니처 확장으로 기존 테스트 영향] → `NewMockStorage`가 기본 no-op `DeleteByURLFunc`를 제공해 기존 테스트는 무수정 통과.
- [CTE 추가로 upsert 쿼리 복잡도 증가] → sqlc 생성 코드로 타입 안전성이 보장되고, 단위 테스트가 신규 insert(NULL prev)·갱신(prev 반환) 두 경로를 고정한다.

## Migration Plan

1. `storage.Client`에 `KeyFromURL`·`UploadWithKey` 추가(D1·D6), `bot.Storage`에 `DeleteByURL` 추가 + 어댑터/목 구현.
2. `pins.sql` 수정 → `sqlc generate` → `UpsertBotPinByURLRow`에 `PrevOgImage` 추가 (기존 호출부는 컬럼 추가만이므로 무영향).
3. `ProcessDocument`에 D3 판정 로직 삽입.
4. 배포는 일반 롤링. 롤백 시 정리만 사라지고 기존 동작(누적 + TTL)으로 복귀 — 데이터 마이그레이션 없음.

## Open Questions

(없음)
