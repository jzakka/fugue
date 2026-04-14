## Context

`make show-map`으로 Pioneer 그래프를 D3.js force-directed graph로 시각화하는 기능이 이미 구현되어 있다. 이번 변경은 노드 타입 체계 단순화, 사이트별 필터링 UI 추가, coverage 지표 제거를 수행한다.

## Goals / Non-Goals

**Goals:**
- 노드 타입을 list/detail 2종으로 단순화하여 Harvester 스크립트 라우팅과 일치시킨다
- 사이트별 필터링으로 한 번에 하나의 사이트 그래프만 탐색할 수 있게 한다
- 불필요한 coverage 지표를 제거하여 UI를 정리한다

**Non-Goals:**
- "전체 보기" 모드 (항상 하나의 사이트가 선택됨)
- 노드 타입 시스템의 근본적 재설계 (classifyURL의 분류 로직 자체는 유지, 반환값만 통합)

## Decisions

### 1. 노드 타입 통합: list + detail

**결정**: listing, gallery, category를 `list`로 통합. `detail`은 유지. `skip`은 기존대로 유지 (classifyURL 외부에서 사용).

| 기존 | 변경 후 | 근거 |
|------|---------|------|
| listing | list | Harvester가 동일한 "목록 파싱" 로직 사용 |
| gallery | list | 동일 |
| category | list | 동일 |
| detail | detail | 콘텐츠 상세 파싱은 구조적으로 다름 |
| skip | skip (유지) | PathPatternFilter에서 참조 |

**classifyURL 변경**: gallery/category 패턴 매칭을 제거하지 않고, 반환값만 `NodeTypeList`로 변경. 패턴 매칭 로직 자체는 향후 세분화 가능성을 위해 보존.

**NodeTypePriority 변경**: list의 priority를 기존 listing과 동일하게 유지하고, gallery/category의 별도 priority를 제거한다.

**DB 기존 데이터**: DB에 이미 저장된 `listing`, `gallery`, `category` 값은 마이그레이션하지 않는다. 시각화 레이어(`template.html`)에서 `listing`/`gallery`/`category`를 모두 `list`로 취급하여 색상/필터링에 반영한다. Go 코드에서는 새 노드만 `list`로 생성하고, 기존 값은 자연적으로 교체될 때까지 유지한다.

**색상 체계**: list는 파란색, detail은 초록색으로 시각적 구분한다.

### 2. 사이트별 필터링 UI

**결정**: Stats 패널에 사이트 리스트를 라디오 버튼 형태로 표시. 하나만 선택 가능. 페이지 로드 시 첫 번째 사이트 자동 선택.

**사이트 항목 표시 형식**:
```
● pixiv        pixiv.net
○ soundcloud   soundcloud.com
○ artstation   artstation.com
```
- 왼쪽: domain에서 TLD 제거한 이름 (bold)
- 오른쪽: 전체 domain (muted)

**필터링 동작**:
- 선택한 사이트의 `site_id`와 일치하는 노드만 표시
- 엣지는 양 끝점이 모두 표시된 노드에 속할 때만 표시
- Stats (Nodes, Edges)는 필터링된 카운트로 업데이트
- D3 simulation을 재시작하여 레이아웃 재계산

### 3. Coverage 삭제

**결정**: Stats 패널에서 script coverage bar, 퍼센트, 관련 코드를 제거.

**유지**: 노드 stroke로 표현하는 script 유무 (implemented=밝은 stroke, missing=어두운 stroke). Legend에서 script status 항목도 유지.

**제거 대상**:
- `template.html`: coverage 통계 표시 요소 및 관련 CSS
- `types.go`: coverage 관련 struct 및 필드
- `repository.go`: coverage 집계 함수 및 로직
- `main.go`: coverage 콘솔 출력 및 필터링 시 coverage 재계산 호출

`CheckScriptExists()`는 노드별 `has_script` 판정에 계속 사용하므로 유지.

## Risks / Trade-offs

**[Trade-off] "전체 보기" 없음**: 사이트 간 cross-site edge를 볼 수 없다. 현재 cross-site edge가 없으므로 문제없음. 향후 필요 시 추가 가능.

**[Trade-off] gallery/category 구분 상실**: 시각화에서 목록 페이지의 세부 유형을 구분할 수 없다. Harvester 라우팅 기준으로는 불필요하므로 수용.
