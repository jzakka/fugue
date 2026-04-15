## Context

Pioneer는 BFS 크롤 중 각 노드 타입에 대해 AI 클라이언트를 호출하여 파싱 스크립트를 생성하고, 이를 `bot_scripts` 테이블에 저장한다. `show-map`은 이 그래프를 D3로 시각화하며, 노드의 스크립트 구현 상태를 색상으로 구분한다.

현재 두 가지 단절이 있다:

1. AI CLI 클라이언트가 `codex`를 bare로 실행한다. `codex`는 인터랙티브 모드가 기본이라 TTY를 요구하며, Go subprocess에서 stdin을 파이프하면 `stdin is not a terminal` 에러가 발생한다.

2. `show-map`의 `CheckScriptExists`가 `apps/api/internal/bot/sources/<domain>/<node_type>.go` 파일의 존재 여부를 확인한다. Pioneer는 스크립트를 DB에 저장하므로, 이 파일은 존재하지 않는다. 결과적으로 모든 노드가 "미구현"으로 표시된다.

## Goals / Non-Goals

**Goals:**
- Pioneer의 AI 스크립트 생성이 로컬 환경에서 정상 동작한다
- `show-map` 시각화에서 DB에 저장된 스크립트의 구현 상태가 정확히 반영된다

**Non-Goals:**
- AI 스크립트의 품질 개선이나 검증 로직 변경
- `show-map` UI/UX 변경 (색상 체계, 레이아웃 등)
- Harvester 워크플로우 수정

## Decisions

### Decision 1: `codex exec -` 서브커맨드 자동 주입

`CLIClient.Call`에서 command가 `codex`일 때 args 앞에 `exec -`를 자동 삽입한다. `exec`은 비인터랙티브 모드이고, `-`는 stdin에서 프롬프트를 읽겠다는 의미다.

대안: `NewCLIClient`에서 struct 필드의 args를 영구 변경하는 방법. 하지만 `Call` 시점에 새 슬라이스를 생성하여 주입하면 struct의 `c.args` 필드가 변경되지 않아, 여러 번 Call해도 동일하게 동작한다.

### Decision 2: `HasScript` 판정을 DB 기반으로 변경

`bot_scripts` 테이블에서 `(site_id, node_type)` 조합을 한 번에 조회하여 맵으로 캐싱한다. `ListScriptKeysForGraph` sqlc 쿼리를 추가하고, `FetchGraphData`에서 노드 빌드 시 이 맵을 참조한다.

대안: 노드별로 `GetScriptBySiteType`을 호출하는 방법. N+1 쿼리 문제가 발생하므로 batch 조회가 적합하다.

### Decision 3: `CheckScriptExists` 함수 및 `ScriptPathTemplate` 상수 제거

파일 시스템 기반 체크는 현재 아키텍처와 맞지 않다. 제거하여 혼란을 방지한다.

## Risks / Trade-offs

- `codex exec` 동작이 `codex` 버전에 따라 다를 수 있다 → `codex exec --help`에서 `-` stdin 지원을 확인했으며, 현재 설치된 버전에서 동작한다.
- `ListScriptKeysForGraph`가 전체 스크립트 키 쌍(site_id, node_type)을 조회한다 → script_code 등 대용량 컬럼은 포함하지 않으므로 현재 규모에서는 문제가 없다. 행 수는 "사이트 수 x 노드 타입 수"로 제한적이다.
