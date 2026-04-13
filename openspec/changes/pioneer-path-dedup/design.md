## Context

현재 Pioneer의 `hashURL()` 함수는 전체 URL 문자열(쿼리 파라미터 포함)을 MD5 해시합니다. 이로 인해 `info.php?id=13404`와 `info.php?id=13533`이 별도 노드로 생성되고, `contest/abc`와 `contest/def`도 별도 노드가 됩니다. 100개 노드 중 상당수가 같은 페이지 템플릿의 반복입니다.

Pioneer가 만드는 그래프는 "사이트의 페이지 타입 구조"를 표현해야 하므로, 같은 패턴의 URL은 하나의 노드로 합쳐야 합니다.

## Goals / Non-Goals

**Goals:**
- 같은 path에 쿼리 파라미터만 다른 URL을 동일 노드로 합침
- path 내 숫자 ID 세그먼트를 `{id}` 플레이스홀더로 치환하여 동일 패턴의 URL을 합침
- 원본 URL(sample) 하나를 보존하여 Harvester가 실제 페이지를 fetch할 수 있도록 함
- DB 마이그레이션으로 `sample_url` 컬럼 추가

**Non-Goals:**
- 기존 DB 데이터 마이그레이션/정리 (재크롤로 자연 전환)
- 사이트별 커스텀 정규화 규칙

## Decisions

### 1. Template path 정규화 함수

**결정**: `templatePath(urlStr string) string` 함수를 도입합니다. 두 단계로 정규화합니다:

> **명명 참고**: 기존 `link_filter.go`에 `canonicalURL()` (트래킹 파라미터 제거 + www 정규화)이 이미 존재하므로, 역할 구분을 위해 `templatePath`로 명명. `templatePath`는 **노드 패턴 매칭 전용** (쿼리 전체 제거 + 숫자 ID 치환).

1. **쿼리 파라미터 + fragment 제거**: `url.Parse` → scheme + host + path만 유지
2. **숫자 ID 치환**: path 내 순수 숫자로만 구성된 세그먼트를 `{id}`로 치환
   - `/artworks/12345678` → `/artworks/{id}`
   - `/info.php` → `/info.php` (변경 없음, 숫자가 아님)
   - `/contest/abc123` → `/contest/abc123` (순수 숫자가 아님, 유지)
   - `/item/12345/comments` → `/item/{id}/comments`

**근거**: 쿼리 파라미터가 콘텐츠를 구분하는 경우(`?id=123`)와 path가 콘텐츠를 구분하는 경우(`/artworks/123`) 모두를 처리합니다. 순수 숫자 세그먼트만 치환하여 `contest/magicalparty` 같은 slug는 보존합니다.

**대안 검토**:
- 쿼리 파라미터만 제거하고 숫자 치환은 하지 않음 → `/artworks/12345`와 `/artworks/67890`이 별도 노드로 남아 중복 문제 절반만 해결됨
- 정규식으로 사이트별 패턴 매칭 → 과도한 복잡성, Non-Goal로 제외

### 2. hashURL 변경

**결정**: 기존 `hashURL(urlStr)` → `hashURL` 내부에서 `templatePath(urlStr)`를 호출하도록 변경합니다. `url_hash` 컬럼에는 template path의 MD5가 저장됩니다.

**영향**: 기존 데이터의 url_hash와 새 해시가 다르므로, 재크롤 시 기존 노드와 매칭되지 않고 새 노드가 생성됩니다. 이는 수용합니다 (기존 데이터는 재크롤로 자연 교체).

### 3. sample_url 컬럼

**결정**: `bot_graph_nodes` 테이블에 `sample_url TEXT` 컬럼을 추가합니다. `url` 필드에는 template path를, `sample_url`에는 최초 발견된 원본 URL을 저장합니다.

**용도**:
- Harvester `executeNode`에서 `node.Url` 대신 `node.SampleUrl`로 실제 HTTP fetch
- Harvester `findRootNode`에서 `hashURL(site.RootUrl)` (내부적으로 `templatePath` 적용)로 조회하도록 변경
- 시각화 tooltip에서 실제 URL 표시

**영향받는 sqlc 쿼리**: `sample_url`을 SELECT에 포함해야 하는 쿼리:
- `CreateNode` (INSERT에 추가)
- `GetNodeByHash`, `GetNodeByURL` (SELECT에 추가)
- `ListNodesBySite`, `ListNodesByType` (SELECT에 추가)
- `ListAllNodesForGraph` (시각화용 SELECT에 추가)

### 4. 노드 중복 판단 흐름

```
URL 발견: https://www.pixiv.net/artworks/12345678

  ┌────────────────────────────────────────────┐
  │ templatePath()                             │
  │ → https://www.pixiv.net/artworks/{id}      │
  └──────────────────┬─────────────────────────┘
                     │
  ┌──────────────────▼─────────────────────────┐
  │ hashURL(canonical)                         │
  │ → md5("https://www.pixiv.net/artworks/{id}") │
  └──────────────────┬─────────────────────────┘
                     │
  ┌──────────────────▼─────────────────────────┐
  │ GetNodeByHash(site_id, hash)               │
  │ → 이미 존재? → edge만 생성                │
  │ → 미존재?   → CreateNode                  │
  │   url: "https://www.pixiv.net/artworks/{id}" │
  │   sample_url: 원본 URL                     │
  └────────────────────────────────────────────┘

※ classifyURL()과 fetchHTML()은 원본 URL을 사용한다.
  templatePath는 노드 식별(해시/저장)에만 적용된다.
```

## Risks / Trade-offs

- **[기존 데이터 비호환]** 새 해시 체계와 기존 url_hash가 다르므로 재크롤 전까지 기존 노드와 새 노드가 공존할 수 있습니다. → **수용**: 재크롤로 자연 전환. `make pioneer SITE=pixiv`로 해결.
- **[과도한 합침]** 숫자 ID 치환이 다른 의미의 path를 합칠 수 있습니다 (예: `/year/2024` → `/year/{id}`). → **수용**: Pioneer의 목적이 사이트 구조 매핑이므로, 대부분의 경우 동일 템플릿으로 봐도 무방합니다.
- **[sample_url 고정]** 최초 발견 URL만 저장하므로, 해당 URL이 404가 되면 Harvester가 실패할 수 있습니다. → **완화**: Harvester에서 fetch 실패 시 다른 URL로 재시도하는 로직은 향후 추가.
