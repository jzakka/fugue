## Why

Pioneer가 크롤한 그래프 구조를 시각화하는 `make show-map` 커맨드가 이미 존재하지만, 몇 가지 개선이 필요하다:

1. **노드 타입 과분류**: listing/gallery/category가 Harvester 파싱 관점에서 동일한 "목록 페이지"임에도 3종으로 나뉘어 시각적 노이즈를 유발
2. **전체 그래프만 표시**: 사이트가 여러 개일 때 한 화면에 전부 보여서 복잡하고 특정 사이트 탐색이 어려움
3. **Script coverage 지표 불필요**: coverage 퍼센트가 현 시점에서 유용한 정보가 아님

## What Changes

- **노드 타입 통합**: listing/gallery/category를 `list`로 통합. 노드 타입은 `list`와 `detail` 2종만 유지
- **사이트별 필터링**: Stats 패널에 사이트 리스트를 표시하고, 선택한 사이트의 노드/엣지만 그래프에 렌더링. 기본 선택은 첫 번째 사이트
- **Coverage 삭제**: Stats 패널에서 script coverage bar 및 퍼센트 제거. 노드 stroke의 script 유무 표시는 유지

## Capabilities

### New Capabilities

_(없음)_

### Modified Capabilities

- `bot`: classifyURL 노드 타입 통합 (listing/gallery/category → list)

## Impact

- **internal/bot/domain.go**: NodeType 상수 정리 (list, detail)
- **internal/bot/pioneer.go**: classifyURL() 반환값 변경
- **internal/bot/pioneer_test.go**: 테스트 기대값 업데이트
- **cmd/bot-visualize/template.html**: 색상 체계, legend, stats 패널, 사이트 필터 UI
- **internal/bot/cmd/visualize/types.go**: coverage 관련 코드 제거
- **internal/bot/cmd/visualize/repository.go**: script 존재 확인 유지, coverage 집계 제거
- **cmd/bot-visualize/main.go**: coverage 관련 출력 및 호출 제거
