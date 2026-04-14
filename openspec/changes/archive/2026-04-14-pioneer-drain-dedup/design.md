## Context

Pioneer 크롤러는 `canonicalPath()`를 통해 순수 숫자 segment만 `{id}`로 치환. 비숫자 파라미터(검색어, 태그명 등)는 각각 별도 노드로 생성된다.

기존 `pioneer-lazy-trie-merge`에서 PathTrie + leaf-only 방식으로 leaf explosion을 처리했으나, mid-path parameterization(`/tags/TAG/artwork`)은 탐지 불가. Subtree fingerprint 확장도 검토했으나 완전 일치만 지원하고 다중 suffix 처리가 복잡.

현재 관련 코드:
- `apps/api/internal/bot/path_trie.go` — PathTrie 구조체, MergeTargets
- `apps/api/internal/bot/trie_merge.go` — RunTrieMerge, mergeOnePrefix
- `apps/api/internal/bot/path_trie_test.go` — 161 tests
- `apps/api/cmd/bot/main.go` — pioneer/merge 서브커맨드
- DB 유니크 제약: `bot_graph_nodes(site_id, url_hash)`

## Goals / Non-Goals

**Goals:**
- Leaf explosion + mid-path parameterization을 통합적으로 탐지
- 정적 리소스 경로(`/api/users/{id}` vs `/api/posts/{id}`) 오탐 방지
- 기존 머지 로직(엣지 재연결, 중복 제거) 재사용
- `make pioneer SITE=xxx` 후 자동 머지 유지

**Non-Goals:**
- Pioneer 크롤 중 실시간 분석 (후처리만)
- `canonicalPath()` 자체 변경
- segment 내부 패턴 탐지 (e.g., `cat-*-sku-*`)

## Decisions

### 1. Drain 알고리즘 채택

**선택**: 로그 파싱 분야에서 검증된 Drain 알고리즘(He et al., ICWS 2017)을 URL path segment 분석에 적용.

**Drain 동작 흐름**:
```
URL 입력: /tags/TAG1/artwork
         ↓
Step 1: path depth로 1차 분류
        depth=3 그룹에 배치
         ↓
Step 2: 첫 번째 segment로 tree 탐색
        "tags" 노드로 라우팅
         ↓
Step 3: 기존 클러스터와 유사도 비교
        [tags, TAG1, artwork] vs [tags, TAG2, artwork]
        유사도 = 동일 토큰 수 / 전체 토큰 수 = 2/3 = 0.67
         ↓
Step 4: threshold 이상이면 같은 클러스터에 합류
        클러스터 템플릿: tags <*> artwork
```

**이유**: (1) 두 패턴을 통합 처리, (2) O(n) 삽입 효율, (3) 학계에서 검증됨, (4) 기존 trie 구조와 유사한 tree 기반.

### 2. Drain 파라미터 설정

**Tree depth**: 1 (root → length_node → clusters)
- segment count로만 1차 분류. first-segment 라우팅은 사용하지 않음.
- 원래 Drain 논문의 depth-2(first-segment 라우팅)를 적용하면 depth-2 leaf explosion(`/howto/a`, `/howto/b`)에서 첫 segment "howto"가 동일하여 라우팅은 되지만, depth-2 URL은 비교 대상 token이 1개뿐이라 클러스터링 정밀도가 떨어짐. depth-1로 단순화하여 segment count 기반 분류 후 유사도 비교로 판정.

**Similarity threshold**: 0.5
- 3-segment URL에서 1개가 다르면 sim=2/3=0.67 → 통과. 2개가 다르면 sim=1/3=0.33 → 차단.
- 보수적인 기본값. CLI 플래그로 오버라이드 가능.

**Count threshold**: 5 (기존과 동일)
- 클러스터 내 URL 수가 5를 초과해야 머지 대상. CLI 플래그로 오버라이드 가능.

### 3. Static Segment 후처리 검증 (오탐 방지)

**문제**: Drain 유사도만으로는 `/api/users/{id}` vs `/api/posts/{id}` 오탐을 방지할 수 없음.

```
/api/users/{id}  → [api, users, {id}]
/api/posts/{id}  → [api, posts, {id}]
유사도 = 2/3 = 0.67 → threshold 0.5 초과 → 같은 클러스터!
→ 오탐: api <*> {id}
```

**선택**: Drain 클러스터링 후, wildcard 위치 이후의 토큰에 static segment가 있는지 검증.

**규칙**:
- Leaf explosion (wildcard가 마지막 위치): count threshold만으로 판정. 추가 검증 불필요.
- Mid-path (wildcard가 중간 위치): wildcard 이후의 토큰 중 하나 이상이 static(non-parameterized)이어야 머지 대상.

```
검증 예시:

tags <*> artwork
  → wildcard 이후: ["artwork"] → static 있음 → 머지 OK ✓

api <*> {id}
  → wildcard 이후: ["{id}"] → static 없음 → 머지 차단 ✗

categories <*> items {id}
  → wildcard 이후: ["items", "{id}"] → "items" static → 머지 OK ✓

users <*> posts recent
  → wildcard 이후: ["posts", "recent"] → 둘 다 static → 머지 OK ✓
```

**대안**: similarity threshold를 0.7 이상으로 높여서 2/3=0.67을 차단
- 기각 이유: leaf explosion 패턴(`/howto/search/<*>`)에서도 sim=2/3이므로 함께 차단됨.

### 4. 기존 코드 마이그레이션 전략

**선택**: `path_trie.go` → `drain.go`로 교체. `trie_merge.go`의 머지 실행 로직은 재사용.

구체적으로:
- `PathTrie`, `MergeTarget`, `MergeTargets()` → `DrainTree`, `DrainCluster`, `Clusters()` 로 교체
- `MergeTarget` 구조체를 확장: `Suffix string` 필드 추가 (mid-path용)
- `RunTrieMerge()` → `RunDrainMerge()` 로 이름 변경. 내부에서 DrainTree 사용
- `mergeOnePrefix()` → 기존 로직 재사용. MergeTarget에서 victim 노드 매칭 시 `Prefix + "/" + paramValue + Suffix` 패턴 사용
- 기존 `path_trie_test.go` 삭제, `drain_test.go`로 대체

**이유**: PathTrie의 leaf-only 탐지는 Drain의 부분집합. 병렬 유지할 이유 없음.

### 5. 템플릿 URL 생성

**선택**: `Prefix + "/{param}" + Suffix` 형식.

- Leaf explosion: `/howto/search/{param}` (Suffix = "")
- Mid-path: `/tags/{param}/artwork` (Suffix = "/artwork")
- Deep mid-path: `/users/{param}/posts/recent` (Suffix = "/posts/recent")

### 6. 다중 suffix 처리

**선택**: Drain이 동일 depth 그룹 내에서 여러 클러스터를 자동 생성.

```
/tags/TAG1/artwork       → 클러스터 A: tags <*> artwork
/tags/TAG1/illustrations → 클러스터 B: tags <*> illustrations

Drain 내부 동작:
1. TAG1/artwork 삽입 → 새 클러스터 A 생성
2. TAG2/artwork 삽입 → A와 sim=2/3 → A에 합류
3. TAG1/illustrations 삽입 → A와 비교: [tags,TAG1,illustrations] vs [tags,<*>,artwork]
   → <*> 위치 제외, "illustrations" vs "artwork" 불일치 → sim < threshold → 새 클러스터 B 생성
4. TAG2/illustrations → B에 합류
```

**Drain의 wildcard 유사도 계산**: `<*>` 위치는 유사도 계산에서 제외(분모·분자 모두 불포함). 따라서 3-token URL에서 wildcard 1개이면 비교 대상은 2개. 1개 일치 → sim = 1/2 = 0.5. wildcard가 있는 템플릿은 strict threshold(`>`, not `>=`)를 적용하여 sim=0.5인 경우 합류를 거부(wildcard creep 방지). wildcard가 없는 템플릿은 `>=`를 적용하여 depth-2 leaf explosion(sim=0.5)을 허용.

## Risks / Trade-offs

- **[Risk] Drain sim threshold가 사이트마다 최적값이 다를 수 있음** → CLI 플래그로 오버라이드 가능. 기본값 0.5는 보수적.
- **[Risk] 기존 leaf-only 머지로 이미 처리된 노드** → Drain 재실행 시 이미 `{param}` 템플릿인 노드는 클러스터에 1개만 존재하므로 count threshold 미달 → 자동 스킵 (멱등).
- **[Trade-off] PathTrie 코드 삭제** → 기존 161개 테스트를 Drain 테스트로 대체해야 함. leaf explosion 커버리지가 줄지 않도록 주의.
- **[Trade-off] Drain은 URL 전용이 아님** → 로그 파싱용 알고리즘을 URL에 적용하므로, URL 특유의 edge case(query parameter, fragment 등)는 canonicalPath()가 이미 처리한다고 가정.

---

## Rejected Alternatives

### 기각: PathTrie leaf-only 방식 (pioneer-lazy-trie-merge Phase 1)

**방식**: trie에서 리프 노드의 형제 수가 임계값을 초과하면 `{param}`으로 치환.

**기각 이유**: mid-path parameterization을 탐지할 수 없음.

```
/tags/
├── TAG1/
│   └── artwork   ← leaf 1개 (threshold 미달)
├── TAG2/
│   └── artwork   ← leaf 1개 (threshold 미달)
...
→ 탐지 불가
```

**구현 상태**: Phase 1(leaf explosion)은 구현 완료. 아카이브: `archive/2026-04-14-pioneer-lazy-trie-merge`

---

### 기각: Subtree Fingerprint + Static Segment 검증 (pioneer-lazy-trie-merge Phase 2)

**방식**: 각 trie 노드의 subtree fingerprint를 계산하고, 동일 fingerprint 형제가 임계값을 초과하면 파라미터로 판별.

**기각 이유**:
1. 완전 일치(exact fingerprint match)만 지원하여, subtree가 약간만 달라도 별도 그룹으로 분리
2. 다중 suffix 처리 시 leaf path enumeration이 필요하여 구조가 복잡
3. Drain이 유사도 기반 부분 매칭을 지원하여 더 넓은 패턴을 잡으면서도 검증된 알고리즘

**설계 문서**: `archive/2026-04-14-pioneer-lazy-trie-merge/design.md` 참조

---

### 기각: DUST (Different URLs with Similar Text)

**방식**: 서로 다른 URL이 동일한 페이지 콘텐츠를 가리키는 경우 치환 규칙을 학습.

**기각 이유**: 페이지 콘텐츠 비교가 필수. Pioneer는 URL과 그래프 구조만 저장하고 콘텐츠 해시를 보관하지 않음.

---

### 기각: Koppula et al. (ML 기반 URL 패턴 학습)

**방식**: 크롤 로그 + 콘텐츠 해시에서 dup cluster를 만들고, ML로 URL 변환 규칙을 일반화.

**기각 이유**: 콘텐츠 의존 + 대규모 크롤 로그 필요. 현재 사이트당 수백 노드 규모에서는 비효율.

---

### 기각: CLUE (URL Distance + DBSCAN)

**방식**: URL 쌍 간 거리를 계산하고 DBSCAN으로 클러스터링.

**기각 이유**: O(n^2) 쌍별 거리 계산. Drain의 tree 구조가 O(n)으로 효율적.

---

### 기각: Agarwal et al. (Deep Tokenization + Token Tree)

**방식**: URL을 구분자 단위로 세밀하게 토큰화하여 token tree 구성.

**기각 이유**: segment 내부 패턴(`cat-*-sku-*`)까지 잡는 세밀함이 과함. Pioneer의 URL은 path segment 단위 파라미터가 주된 패턴.
