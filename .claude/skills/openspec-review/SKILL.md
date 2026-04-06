---
name: openspec-review
description: Review an OpenSpec change for design correctness, implementation feasibility, document consistency, and completeness. Use when the user wants a thorough review of proposal, design, tasks, and specs before implementation.
license: MIT
compatibility: Requires openspec directory structure.
metadata:
  author: openspec
  version: "1.0"
---

OpenSpec change의 설계 문서를 심층 리뷰한다.

**Input**: change 이름 또는 경로. 없으면 질문한다.

---

## Steps

### 1. Change 디렉토리 확인

```bash
ls openspec/changes/<name>/
```

필수 파일: `proposal.md`, `design.md`, `tasks.md`, `specs/` 디렉토리.
없는 파일이 있으면 보고하고 있는 파일만으로 진행.

### 2. 모든 문서 읽기

- `proposal.md` — Why, What Changes, Capabilities, Impact
- `design.md` — Context, Goals, Decisions, Risks
- `tasks.md` — 구현 단계
- `specs/**/spec.md` — 요구사항 시나리오

### 3. 6가지 관점으로 리뷰

#### 3.1 문서 간 일관성

- proposal.md의 "What Changes"와 design.md의 Decisions가 일치하는가
- proposal.md의 Impact와 tasks.md의 작업 범위가 일치하는가
- design.md의 결정 사항이 spec.md의 요구사항을 충족하는가
- 대안 검토에서 기각된 방식이 다른 문서에서 여전히 언급되고 있지 않은가

#### 3.2 설계 결정의 구현 가능성

- 사용하려는 라이브러리/API의 실제 동작과 설계가 일치하는가
- SQL 쿼리가 실제 실행 가능한가 (PostgreSQL 문법, 인덱스 활용)
- Go 코드 예시가 컴파일 가능한가 (인터페이스, 타입, import)
- 프론트엔드 코드가 Next.js App Router 패턴과 일치하는가
- Edge case 처리가 누락되지 않았는가

#### 3.3 Tasks 완전성

- 설계의 모든 결정이 task로 반영되었는가
- sqlc generate, 의존성 추가, 테스트 등 부수 작업이 누락되지 않았는가
- task 간 의존 순서가 올바른가
- 기존 테스트에 대한 영향 확인이 포함되어 있는가

#### 3.4 Spec 정확성

- **spec.md가 유효한 delta spec 형식인가** — `## ADDED Requirements`, `## MODIFIED Requirements`, `## REMOVED Requirements` 섹션으로 구성되어야 한다. HTML 코멘트만 있거나 빈 파일이면 무효. 변경할 요구사항이 없으면 spec 파일 자체가 없어야 한다.
- **Capability 배치가 올바른가** — `openspec/specs/` 디렉토리를 실제로 조회하여 기존 도메인 목록을 확인한다. proposal.md에서 New Capability로 선언된 스펙이 기존 도메인의 범위에 속하면 반드시 Modified Capabilities로 해당 도메인 스펙에 통합해야 한다. 구체적 검증 절차:
  - `openspec/specs/` 하위 디렉토리를 나열하고, 새 스펙의 행위 주체(actor)나 대상(entity)이 기존 도메인의 엔티티/화면/API와 겹치는지 판단한다. 겹치면 도메인 중복이다.
  - 새 도메인이 정당화되려면 (1) 기존 도메인 어디에도 속하지 않고, (2) 독립된 바운디드 컨텍스트를 형성하며, (3) 최소 3개 이상의 독립 요구사항이 예상되어야 한다.
  - 도메인 겹침이 발견되면 **HIGH** 우선순위 이슈로 보고하고, 어떤 기존 도메인에 Modified Capability로 통합해야 하는지 구체적으로 명시한다.
- **행위 계약(behavior contract)인가** — 스펙은 외부에서 관찰 가능한 행위를 기술해야 한다. 구현이 바뀌어도 외부 행위가 동일하면 스펙은 변하지 않아야 한다. 아래 구현 세부사항이 스펙에 포함되어 있으면 지적한다:
  - CSS 클래스명, hex 색상, Tailwind 유틸리티 (예: `bg-white`, `text-accent`, `px-4`)
  - 컴포넌트 props 인터페이스 (예: `{ tags: PopularTag[], onSelect: () => void }`)
  - API 응답 JSON 필드명 (예: `has_more`, `next_cursor`) — 행위적 기술로 대체해야 함
  - 내부 에러코드 — 에러 조건을 서술적으로 기술해야 함
  - SQL 쿼리, Go struct 필드명, DB 컬럼명
- 시나리오의 WHEN/THEN이 구현될 동작을 정확히 기술하는가
- 설계에서 수용한 trade-off가 spec에 반영되어 있는가
- 용어가 설계와 일관되는가

#### 3.5 누락된 고려사항

- 에러 처리/장애 시나리오가 다뤄졌는가
- 기존 코드와의 하위 호환성이 유지되는가
- Go 패키지 경계를 위반하지 않는가
- 마이그레이션/롤백 전략이 필요한가

#### 3.6 비교 연산자, 네이밍, off-by-one 등 세부 정확성

- 반환값 비교 연산자가 올바른가 (`>` vs `>=`)
- 인터페이스 메서드 시그니처가 일관되는가
- 패키지 위치가 프로젝트 구조와 일치하는가
- 설정 키/환경변수 이름이 일관되는가

### 4. 결과 보고

심각도별로 정리:

```
| 우선순위 | # | 항목 |
|---------|---|------|
| MEDIUM  | 1 | 이슈 제목 |
| LOW     | 2 | 이슈 제목 |
```

각 이슈에 대해:
- 어떤 문서의 몇 번째 줄이 문제인지
- 왜 문제인지
- 어떻게 수정하면 되는지

이슈가 없으면 "지적 사항 없습니다. 구현 진행해도 좋습니다." 출력.

---

## Guardrails

- **읽기 전용** — 코드나 문서를 수정하지 않는다. 리뷰 결과만 보고한다.
- **거짓 긍정 방지** — 확실하지 않은 이슈는 "확인 필요"로 표시한다. 추측으로 이슈를 만들지 않는다.
- **설계 취향 강요 금지** — "이렇게 하는 게 더 낫다"가 아니라 "이렇게 하면 이런 문제가 발생한다"를 근거와 함께 제시한다.
- **문서 의도 존중** — design.md에서 의도적으로 선택한 trade-off를 이슈로 올리지 않는다. 단, 그 trade-off가 문서 내에서 모순되거나 spec과 불일치하면 지적한다.
