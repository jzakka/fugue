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

## 워크플로우 규칙

- 커밋 전에 반드시 `/codex` review를 실행할 것
