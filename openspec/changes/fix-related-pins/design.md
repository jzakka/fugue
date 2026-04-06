## Context

연관 핀 SQL 쿼리(`RelatedPins`)의 ORDER BY 절에서 `p.tags & $2::text[]`를 사용한다. `&`는 PostgreSQL `intarray` 확장의 정수 배열 전용 연산자이며, text 배열에는 정의되지 않아 매 실행마다 런타임 에러가 발생한다.

WHERE 절의 `p.tags && $2::text[]` (overlap 연산자)는 정상 동작하므로, ORDER BY의 태그 일치 개수 계산 부분만 수정하면 된다.

## Goals / Non-Goals

**Goals:**
- 연관 핀 쿼리가 에러 없이 실행되도록 수정
- 태그 일치 개수 기반 정렬 로직 유지 (일치 태그가 많을수록 상위)

**Non-Goals:**
- 연관 핀 알고리즘 변경 (분야 우선, 태그 일치순, 최대 10개 — 기존 스펙 유지)
- 프론트엔드 변경
- API 응답 형식 변경

## Decisions

### text 배열 교집합 크기 계산 방식

**선택:** `cardinality(ARRAY(SELECT unnest(p.tags) INTERSECT SELECT unnest($2::text[])))`

**대안 1:** `intarray` 확장 설치 후 `&` 사용
- 장점: 간결한 문법
- 단점: 추가 확장 의존성, text 배열에는 여전히 사용 불가 (integer만 지원)

**대안 2:** 서브쿼리로 교집합 크기 계산
- `(SELECT count(*) FROM unnest(p.tags) t WHERE t = ANY($2::text[]))`
- 장점: 간결
- 단점: `= ANY` 대신 `INTERSECT`가 의미적으로 더 명확

**근거:** 대안 2가 실행 계획상 동일하지만, 선택한 방식이 "교집합의 크기"라는 의도를 더 명확히 표현한다. 추가 확장 없이 순수 SQL로 해결.

## Risks / Trade-offs

- **성능**: `unnest + INTERSECT` 서브쿼리가 행마다 실행되지만, LIMIT 10 + GIN 인덱스(`tags`)로 후보 자체가 적어 실질적 영향 미미
- **롤백**: SQL 쿼리 변경 + sqlc 재생성만이므로, git revert로 즉시 복구 가능
