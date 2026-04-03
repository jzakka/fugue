# Fugue - AGENTS.md

## 작업 관리

사용자가 "다음에 뭐 해야 해?", "구현할 거 뭐 있어?", "태스크 알려줘" 등 구현할 작업을 물어보면
반드시 `tasks/` 폴더를 참조하여 답변할 것.

- [태스크 목록](tasks/README.md)
- Phase 1 (기반): [tasks/phase1-foundation/](tasks/phase1-foundation/)
- Phase 2 (보드): [tasks/phase2-boards/](tasks/phase2-boards/)
- Phase 3 (추천): [tasks/phase3-recommendation/](tasks/phase3-recommendation/)

각 태스크 파일에 상태(`[ ]` 미착수, `[~]` 진행 중, `[x]` 완료), 의존성, 영향 범위가 명시되어 있다.
작업 완료 시 해당 태스크 파일의 상태를 업데이트할 것.

## 설계 문서

상세 설계 문서는 docs/ 참조.

- [PRD (ko)](docs/ko/PRD.md)
- [기술 스택](docs/tech-stack.md)
- [MVP 기능 스펙](docs/mvp-features.md)
- [API 엔드포인트](docs/api-endpoints.md)
- [ERD](docs/erd.md)
- [Architecture (앱)](docs/architecture.md)
- [Architecture (인프라, ko)](docs/ko/architecture.md)

## 스펙 작성 규칙

스펙은 **행위 계약(behavior contract)** 이다. 구현이 바뀌어도 외부 관찰 가능한 행위가 동일하면 스펙은 변하지 않아야 한다.

### 스펙에 포함하면 안 되는 것 (구현 세부사항)

| 카테고리 | 나쁜 예 (스펙에 쓰면 안됨) | 좋은 예 (스펙에 쓸 것) |
|---------|------|------|
| CSS/스타일 | `bg-[#0f0f0f]`, `rounded-full` | "다크 테마 배경, 둥근 태그" |
| 컴포넌트 Props | `{ url: string, field: string }` | "URL과 분야를 입력받아 핀 생성" |
| API 필드명 | `og_image`, `creator_id` | "OG 썸네일", "핀한 유저" |
| 에러코드 | `400 BadRequest`, `SSRF_BLOCKED` | "유효하지 않은 URL 오류" |
| DB/설정 | `board_pins.board_id`, `interactions.type` | "보드에 핀 소속", "행동 유형별 기록" |
| 클래스/함수명 | `OGService.Fetch`, `WorksQuerier` | "OG 메타데이터 조회 서비스", "작품 쿼리 인터페이스" |
| 레이아웃 | "2열 Masonry 그리드", "chip 입력" | "카드 그리드로 작품 표시", "태그 복수 입력" |

### 검증

- "구현 기술이 바뀌어도 이 스펙은 여전히 유효한가?" — 그렇다면 좋은 스펙
- Go가 Rust로 바뀌어도, Next.js가 SvelteKit으로 바뀌어도 스펙이 유효해야 한다

### 도메인 스펙 통합 원칙

변경 사항이 기존 도메인의 범위에 속하면 해당 도메인의 스펙에 요구사항을 추가한다. 도메인당 하나의 스펙을 유지하는 것이 원칙이다.

| 도메인 | 범위 |
|--------|------|
| `auth` | 소셜 로그인, JWT, 세션 관리 |
| `pin` | 핀(작품) 생성, 조회, 삭제, OG fetch |
| `board` | 보드 CRUD, 핀-보드 관계 |
| `feed` | 추천 피드, 연관 작품 |
| `interaction` | 암묵적 행동 기록 (view, pin, board_add) |
| `profile` | 유저 계정 (닉네임, 아바타) |

**새 도메인 생성 기준** — 다음 조건을 모두 충족할 때만:
1. 위 도메인 어디에도 행위가 포함되지 않는다
2. 독립된 엔티티 또는 바운디드 컨텍스트를 형성한다
3. 최소 3개 이상의 독립 요구사항이 예상된다

## 워크플로우 규칙

- 커밋 전에 반드시 `/codex` review를 실행할 것
