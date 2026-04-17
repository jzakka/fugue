## REMOVED Requirements

### Requirement: BFS로 사이트를 탐색한다 (Pioneer)
**Reason**: "BFS로 사이트를 탐색"한다는 표현은 Pioneer가 사이트 루트를 시작점으로 사이트 내부를 너비 우선으로 하강한다는 **단일 프로세스/인메모리 세션** 모델을 전제한다. Pioneer는 이제 `URLScheduler`의 얇은 consumer로 재정의되며, 다음 URL은 frontier에서 `Dequeue`로 받고 링크를 `Enqueue`로 되돌려놓는다. 큐 자료구조와 순회 전략은 `URLScheduler` 구현체와 frontier의 우선순위 스코어가 결정하므로, Pioneer 자체의 요구사항으로 "BFS"를 고정하는 것은 타당하지 않다. 또한 기존 시나리오 중 "DOM 기반 링크 추출"과 "필터 체인을 통한 링크 필터링"은 여전히 유효하지만 별도 requirement로 이미 존재하며(링크 추출/필터 체인 관련 requirement들), "이미 방문한 링크에 대한 엣지 생성"과 "복합 우선순위 계산"은 각각 graph-maintenance/scheduler priority-scoring 성격이므로 다른 change에서 재정의되어야 한다.

**Migration**: 본 change가 정의하는 `pioneer` capability의 "Pioneer 메인 루프는 Dequeue → fetch → parse → Enqueue 반복이다" 및 관련 requirement 집합이 Pioneer의 탐색 모델을 대체한다. DOM 기반 링크 추출과 필터 체인 적용은 `bot` spec의 "DOM ancestor selector를 포함하여 링크를 추출한다" 및 필터 관련 requirement들이 계속 담당한다. 복합 우선순위 계산과 사이트당 노드 수 제한은 각각 `pioneer-link-filter-policy`와 `pioneer-worker-budget`에서 재정의된다.
