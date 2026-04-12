## Why

Pioneer가 URL을 노드로 변환할 때 전체 URL 문자열(쿼리 파라미터 포함)을 MD5 해시하여 중복 판단에 사용합니다. 이로 인해 같은 페이지 템플릿의 다른 콘텐츠가 모두 별도 노드로 생성됩니다.

예시 (Pixiv 크롤 결과):
- `info.php?id=13404` → 노드 A
- `info.php?id=13533` → 노드 B
- `info.php?id=13546` → 노드 C
- ... (총 10개 이상)

Pioneer의 목적은 **사이트의 페이지 타입 구조를 매핑**하는 것이지 개별 콘텐츠를 기록하는 것이 아닙니다. 같은 path 패턴의 페이지는 하나의 노드로 합쳐야 그래프가 사이트 구조를 의미 있게 표현합니다.

## What Changes

- **URL 정규화 함수 도입**: Pioneer가 노드 생성/조회 시 사용하는 URL 해시를 "canonical path" 기반으로 변경합니다. 두 가지 정규화 규칙을 적용합니다:
  1. **쿼리 파라미터 제거**: `aaa/bbb?x=1`과 `aaa/bbb?x=2` → 동일 노드 `aaa/bbb`
  2. **숫자 ID 패턴 치환**: `aaa/123`과 `aaa/456` → 동일 노드 `aaa/{id}`
- **노드 URL 필드 변경**: `url` 필드에 원본 URL 대신 canonical path를 저장하여 노드가 "페이지 템플릿"을 표현하도록 합니다. 원본 URL은 최초 발견된 URL 하나를 `sample_url`로 보존합니다.
- **DB 마이그레이션**: `bot_graph_nodes` 테이블에 `sample_url` 컬럼 추가
- **기존 데이터 호환**: 재크롤 시 자연스럽게 새 해시 체계로 전환됩니다. 기존 노드는 그대로 유지됩니다.

## Capabilities

### New Capabilities

_(해당 없음)_

### Modified Capabilities

- `bot`: Pioneer의 URL 중복 판단 로직을 canonical path 기반으로 변경 (기존 스펙의 "그래프 노드와 엣지를 관리한다" 요구사항 중 "새 URL 추가"와 "중복 URL 거부" 시나리오에 해당)

## Impact

- **Pioneer (`apps/api/internal/bot/pioneer.go`)**: `hashURL()` 함수를 `canonicalPath()` + 해시로 교체, `crawl()` 루프에서 canonical URL 사용
- **DB 마이그레이션**: `bot_graph_nodes`에 `sample_url TEXT` 컬럼 추가
- **sqlc 쿼리**: `CreateNode` 파라미터에 `sample_url` 추가
- **시각화**: 노드 tooltip에서 `sample_url`을 표시하여 실제 URL 확인 가능
- **Harvester**: 노드의 `sample_url`을 사용하여 실제 페이지를 fetch
