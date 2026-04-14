## Context

Pioneer 크롤러는 `templatePath()`를 통해 URL을 정규화하고 `url_hash`로 노드를 dedup한다. 현재는 순수 숫자 segment만 `{id}`로 치환하므로, 비숫자 파라미터(검색어, 태그명, 유저 슬러그 등)가 각각 별도 노드로 생성된다. pixiv.net 크롤에서 `/howto/search/` 하위에만 238개 중복 노드가 존재한다.

단일 URL만으로는 마지막 segment가 경로인지 파라미터인지 판단 불가. 크롤된 URL 집합 전체의 통계를 분석해야 한다.

현재 관련 코드:
- `apps/api/internal/bot/pioneer.go` - `templatePath()`, `hashURL()`, 크롤 루프
- `apps/api/internal/db/query/bot.sql` - 노드/엣지 CRUD 쿼리
- `apps/api/cmd/bot/main.go` - CLI 엔트리포인트
- DB 유니크 제약: `bot_graph_nodes(site_id, url_hash)`

## Goals / Non-Goals

**Goals:**
- 크롤 후 같은 path prefix 아래 다수의 리프 노드를 `{param}`으로 자동 머지
- 자식이 있는 segment는 경로로 판단하여 보호 (오판 방지)
- 머지 시 엣지 재연결 및 중복 엣지 제거
- `make pioneer SITE=xxx` 실행 후 자동으로 머지 트리거

**Non-Goals:**
- Pioneer 크롤 중 실시간 trie 분석 (후처리만)
- `templatePath()` 함수 자체의 변경 (기존 숫자 dedup 로직 유지)
- 크로스 사이트 trie 분석 (사이트별 독립 처리)

## Decisions

### 1. 후처리 방식 (크롤 후 분석 + 머지)

**선택**: 크롤 완료 후 DB 데이터로 trie를 빌드하고, 리프 형제 수 > 임계값인 레벨만 머지

**대안 A**: 크롤 중 실시간 trie 분석
- 기각 이유: 임계값 도달 전에 이미 생성된 노드의 롤백 필요. 또한 `/aaa/bbb`를 파라미터로 머지한 뒤 `/aaa/bbb/254`가 발견되면 `bbb`가 사실 경로였음을 알게 되나 복구 불가.

**대안 B**: known prefix 하드코딩 (search, tag, users 등)
- 기각 이유: 사이트마다 URL 구조가 달라 확장성 부족. 유지보수 부담.

### 2. 머지 대상: 리프 노드 + Subtree Fingerprint 기반 mid-path

**선택**: 두 가지 탐지 경로를 모두 적용

**경로 A — Leaf Explosion (기존)**: trie에서 자식이 없는(리프) 노드이면서 형제 수가 임계값을 초과하는 경우 `{param}`으로 치환.

**경로 B — Mid-Path Parameterization (신규)**: 비리프 형제들의 subtree fingerprint를 계산하여, 동일 fingerprint를 가진 형제 수가 임계값을 초과하면 해당 레벨을 `{param}`으로 판별. 단, static segment 검증(아래 Decision 2-1)을 통과해야 함.

**이유**: Leaf explosion만으로는 `/tags/TAG1/artwork`, `/tags/TAG2/artwork` 같은 mid-path parameterization을 탐지할 수 없음. 변수가 경로 중간에 위치하고 뒤에 고정 suffix가 오는 패턴에서 각 TAG 노드 아래 리프가 1개뿐이라 기존 threshold에 걸리지 않음.

```
현재 trie에서 놓치는 구조:

/tags/
├── TAG1/
│   └── artwork   (leaf 1개 — threshold 미달)
├── TAG2/
│   └── artwork   (leaf 1개 — threshold 미달)
...50개...

→ tags/ 아래 children은 non-leaf → 기존 로직 완전히 통과
→ Subtree fingerprint로 "artwork:∅" 동일 형제 50개 탐지
```

### 2-1. Static Segment 검증 (오탐 방지)

**선택**: subtree fingerprint 그룹이 threshold를 초과하더라도, 해당 subtree에 `{}`로 감싸지지 않은 static segment가 1개 이상 포함된 경우에만 머지 대상으로 인정.

**이유**: `/api/users/{id}`, `/api/posts/{id}` 같은 패턴에서 `users`와 `posts`는 서로 다른 리소스이나, subtree fingerprint가 `"{id}:∅"`로 동일. API 라우트가 6개 이상이면 오탐 발생 가능. Static segment 검증으로 차단:

```
머지 대상 O:
  /tags/TAG1/artwork     → subtree에 "artwork" (static) 포함
  /categories/C1/items/{id} → subtree에 "items" (static) 포함

머지 대상 X:
  /api/users/{id}        → subtree에 "{id}" (parameterized)만 존재
  /api/posts/{id}        → 동일. static segment 없음 → 제외
```

**대안 A**: fingerprint만으로 판단 (static 검증 없이)
- 기각 이유: API route 오탐 위험. 사이트에 따라 동일 구조의 경로가 6개 이상 존재 가능.

**대안 B**: non-leaf 형제 수만 카운트 (fingerprint 없이)
- 기각 이유: 서로 다른 subtree 구조를 가진 형제까지 한꺼번에 카운트되어 오탐 폭증.

### 2-2. Subtree Fingerprint 계산 방식

**선택**: 재귀적으로 자식 이름을 정렬하여 canonical string을 생성

```
fingerprint(leaf)     = "∅"
fingerprint(node)     = sorted children을 "name(fp)" 형태로 join
                        예: "artwork(∅)" 또는 "posts(recent(∅))"
```

동일 fingerprint = 동일한 하위 경로 구조. 정렬하므로 삽입 순서에 무관.

### 2-3. MergeTarget 구조 확장

**선택**: `MergeTarget`에 `Suffix` 필드를 추가하여 leaf explosion과 mid-path를 통합 표현

```
type MergeTarget struct {
    Prefix      string   // 부모 prefix: "/tags"
    ParamValues []string // 파라미터 값: ["TAG1", "TAG2", ...]
    Suffix      string   // mid-path suffix: "/artwork" (leaf explosion이면 "")
}
```

템플릿 URL = `Prefix + "/{param}" + Suffix`

머지 시 victim 노드 매칭: URL path가 `Prefix + "/" + paramValue + Suffix`인 노드

### 3. 임계값: 사이트별 설정 가능, 기본값 5

**선택**: `bot_sites` 테이블에 설정을 저장하지 않고, 코드 상수로 기본값 5 사용. CLI 플래그로 오버라이드 가능.

**이유**: 아직 사이트별로 다른 임계값이 필요한 요구사항 없음. YAGNI.

### 4. DB 머지 전략: UPDATE + 중복 엣지 정리

**선택**: 대표 노드 1개를 남기고 나머지는 엣지 재연결 후 DELETE. 각 prefix를 하나의 트랜잭션으로 처리.

구체적 순서:
1. Trie 분석으로 머지 대상 prefix 목록 추출
2. 각 prefix에 대해 (트랜잭션 내):
   a. 대표 노드 1개 선택 (가장 오래된 created_at)
   b. 재연결 시 중복이 되는 엣지를 먼저 삭제 (예: X→A, X→B가 있고 A를 B로 머지하면 X→B가 중복. X→A를 먼저 DELETE)
   c. 나머지 노드를 참조하는 엣지를 대표 노드로 UPDATE (from_node_id, to_node_id 모두)
   d. self-loop 엣지 삭제 (from_node_id = to_node_id)
   e. 나머지 노드 DELETE (CASCADE로 남은 참조 엣지도 정리)
   f. 대표 노드의 url, url_hash를 `{param}` 템플릿 패턴으로 UPDATE

`bot_graph_edges`에 `UNIQUE(from_node_id, to_node_id)` 제약이 있으므로, 재연결 전에 중복이 될 엣지를 먼저 제거해야 UPDATE가 실패하지 않음.

**대안**: 전체 DELETE 후 새 노드 INSERT
- 기각 이유: 기존 노드 ID를 참조하는 다른 테이블(향후 확장)이 있을 수 있음. UPDATE가 더 안전.

### 5. 실행 시점: Pioneer 크롤 완료 후 자동 실행

**선택**: `cmd/bot/main.go`의 pioneer 서브커맨드에서 크롤 루프 종료 후 trie merge 함수 호출.

별도 `merge` 서브커맨드도 제공하여 수동 실행 가능:
```
go run cmd/bot/main.go merge <site>
```

## Risks / Trade-offs

- **[Risk] 머지 후 sample_url 손실** -> 대표 노드의 sample_url은 보존. 나머지 노드의 sample_url은 손실되지만, 하나만 있으면 충분 (시각화에서 하나의 예시 URL만 표시).
- **[Risk] 임계값이 너무 낮으면 오탐** -> 기본값 5는 보수적. 실제로 5개 이상의 형제가 모두 정적 경로인 사이트는 드뭄. CLI 플래그로 조정 가능.
- **[Risk] 대량 머지 시 DB 성능** -> 사이트당 수백 노드 수준이라 단일 트랜잭션으로 처리 가능. 수만 노드가 되면 배치 처리 필요하나 현재 규모에서는 불필요.
- **[Trade-off] 크롤 중에는 중복 노드 존재** -> 후처리이므로 크롤 완료 전까지는 중복 노드가 그대로. 크롤 효율(큐에 같은 패턴 URL이 많이 쌓임)에는 불리하나, `visited` 맵이 이미 URL 해시 기반으로 중복 방문을 방지하므로 실제 fetch 횟수에는 영향 없음.
