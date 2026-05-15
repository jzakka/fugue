# Design

## Context

`GET /api/feed` 개인화 분기(`authenticated && pinCount >= 10`)의 페이지네이션은 cursor의 offset 값을 추출만 하고 underlying 쿼리 어디에도 전달하지 않는다. 결과적으로 next_cursor를 따라가도 같은 작품이 반복 반환된다.

기존 단일 페이지 응답 구조는:
1. `RecommendByTags(tagIDs, creatorID, recLimit)` → 태그 매칭 추천 ≤ ceil(limit/2)+1
2. (recRows < recLimit이면) `RecommendByMediaType(types, creatorID, deficit)` → 미디어타입 추천으로 보충
3. `ListPinsWithCreator(Offset=0, latestLimit)` → 최신순 작품 latestLimit개
4. `interleave(recommended, latest, limit)` → 교차 배치
5. `ListPinsWithCreator(Offset=len(latestRows), deficit)` → 부족분 추가

각 호출은 페이지 offset을 고려하지 않으므로 페이지 2 이상에서도 동일한 결과를 반환한다.

## Decisions

### Decision 1: offset 기반 페이지네이션을 유지하고 각 underlying 쿼리에 페이지 offset을 그대로 전달한다

**왜**: 현재 cursor 포맷이 `base64("offset:N")`로 정해져 있고, cold-start/unauth 분기(`buildLatestFeed`)는 이미 offset을 정상 전달한다. cursor payload를 확장(seen-id 목록 등)하면 모든 호출자(웹·앱·외부 클라이언트)에 영향이 가고 backward compatibility 부담이 커진다. 페이지 offset을 세 underlying 쿼리(`RecommendByTags`, `RecommendByMediaType`, `ListPinsWithCreator` latest, fill-gap)에 동일하게 전달하는 것이 변경 폭이 가장 작다.

**Trade-off**: 각 쿼리가 독립적으로 offset을 적용하므로 페이지 간에 작품 ID가 100% disjoint하지는 않을 가능성이 있다(예: 태그 매칭 쿼리가 페이지 2에서 추가된 새 핀을 페이지 1의 latest로 다시 만남). 그러나 단조 정렬(`ORDER BY ... created_at DESC, id DESC`) 위에서 page offset이 일관되게 적용되므로 일반적인 사용 패턴(짧은 시간 내 연속 페이지 요청)에서 페이지 1·2의 교집합은 비어 있게 된다. 이 점이 본 Scenario의 "이전 페이지에 포함되지 않은 작품"을 enforce하는 데 충분하다.

거부된 대안:
- 옵션 B(seen-id 기반 cursor): cursor에 직전 페이지의 pin ID 집합을 포함시켜 다음 페이지가 그것을 제외하도록 한다. 정확하지만 cursor 크기가 page 수에 비례해 증가하고, 기존 base64 offset cursor와 호환되지 않는다. 본 결함을 enforce하는 데 필요한 최소 변경 폭을 초과한다.

### Decision 2: 세 underlying 쿼리에 동일한 page offset을 전달한다

**왜**: 세 쿼리는 각자 다른 ORDER BY를 사용한다(`RecommendByTags`는 태그 매칭 카운트 + created_at, `RecommendByMediaType`은 created_at, `ListPinsWithCreator`는 created_at). 각 쿼리 내부에서 단조 페이지네이션이 성립하면 충분하다. 페이지 offset을 그대로 전달하면 각 쿼리의 정렬 순서대로 동일한 위치에서 잘려나간다.

**Trade-off**: fill-gap은 latest 소스에서 추가로 가져오는 보충 호출이므로 페이지 offset에 `len(latestRows)`만큼을 더해야 한다(이미 가져온 latestRows를 두 번 가져오지 않기 위해). 이 보정 로직은 본 변경에서도 유지한다.

거부된 대안:
- recommended 소스에 `offset/2`, latest 소스에 `offset/2`를 주는 분할 방식: limit이 홀수이거나 recommended가 부족해 latest로 채워진 페이지의 다음 페이지를 호출할 때 cursor 의미가 모호해진다. 단순한 한 가지 offset이 더 견고하다.

### Decision 3: 신규 Requirement는 기존 Scenario "피드 페이지네이션"을 강화한다

**왜**: 기존 Scenario는 "이전 페이지에 포함되지 않은 작품이 반환된다"는 SHALL을 이미 명시한다. 그러나 cursor 모델과 underlying 쿼리의 offset 전파 의무가 spec에 묶여 있지 않아 production에서 무방비로 풀린다. 신규 Requirement는 production 코드가 cursor의 offset을 underlying 쿼리에 일관되게 전파해야 한다는 wiring 계약을 명시한다. 기존 Scenario의 의미는 변경하지 않는다.

이 패턴은 직전 cycles에서 다룬 `피드 라우트는 선택적 인증 미들웨어로 보호된다`·`공개 보드 조회 라우트는 선택적 인증 미들웨어로 보호된다`·`공개 프로필 조회 응답에 보드 요약과 핀 요약을 포함한다`와 동일하다. 기존 Scenario를 그대로 두고, production wiring을 명시하는 새 Requirement를 추가하는 형태.

### Decision 4: 본 변경은 페이지 내 작품 중복(within-page dedup)은 다루지 않는다

**왜**: 본 결함은 페이지 1과 페이지 2 간 작품 중복이다. 별개 결함으로, `interleave`가 recommended + latest 양쪽에 같은 작품이 들어가는 경우 중복을 막지 않는 것은 다른 Scenario(혹은 새로운 Requirement)의 대상이다. 본 변경의 범위를 페이지 간 중복으로 한정해 변경 폭을 최소화한다. within-page dedup은 별도 백로그 후보로 만들 수 있다.

### Decision 5: 캐시 키는 변경하지 않는다

**왜**: 캐시 키 `feed:{userID}:{limit}:{offset}`는 이미 offset을 포함하므로 페이지마다 별개 키가 된다. 본 변경 후에는 각 캐시 항목이 실제로 다른 내용을 담게 되어 캐시 의미가 정상화된다. 캐시 TTL 5분이므로 단기 stale 가능성은 있으나 spec이 실시간 갱신을 요구하지 않는다.

### Decision 6: 회귀 방지를 위해 두 페이지 응답의 작품 ID 교집합을 직접 검증하는 테스트를 추가한다

**왜**: 단위 테스트로 enforce하는 가장 명확한 invariant이다. mockQuerier가 `Offset` 파라미터에 따라 서로 다른 결과 슬라이스를 반환하도록 구성하면 페이지 offset이 underlying 쿼리에 전파되는지 직접 확인할 수 있다.
