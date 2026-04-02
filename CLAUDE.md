# Fugue

크로스미디어 창작물 큐레이션 플랫폼. Pinterest가 이미지만이라면, Fugue는 음악/일러스트/영상/글/코드를 분야를 넘나들며 한곳에서 발견한다.

마스코트: 헤드셋 끼고 붓 든 복어 (Fugue ≈ Fugu)

## 문서 구조

- [AGENTS.md](AGENTS.md) - 기술 스택, MVP 기능 스펙, API 엔드포인트
  - [docs/erd.md](docs/erd.md) - DB 스키마 설계
  - [docs/architecture.md](docs/architecture.md) - 시스템 아키텍처

## 프로젝트 구조

```
fugue/
├── apps/
│   ├── api/          # Go Backend
│   └── web/          # Next.js Frontend
├── docs/             # 설계 문서 (ERD, Architecture)
├── helm/
│   └── fugue/
├── terraform/
├── docker-compose.yml
├── CLAUDE.md         # 이 파일 (개요 + 컨벤션)
└── AGENTS.md         # 상세 스펙
```

## 개발 가이드

### 로컬 실행

```bash
docker-compose up -d     # PostgreSQL + Redis
cd apps/api && go run cmd/server/main.go
cd apps/web && npm run dev
```

### 코드 컨벤션

- Go: 표준 프로젝트 레이아웃 (cmd/ internal/)
- Go router: Chi
- Go DB: sqlc (SQL 직접 작성 → Go 코드 자동 생성)
- Frontend: Next.js App Router, TypeScript

## Design System
Always read DESIGN.md before making any visual or UI decisions.
All font choices, colors, spacing, and aesthetic direction are defined there.
Do not deviate without explicit user approval.
In QA mode, flag any code that doesn't match DESIGN.md.

## Skill routing

When the user's request matches an available skill, ALWAYS invoke it using the Skill
tool as your FIRST action. Do NOT answer directly, do NOT use other tools first.
The skill has specialized workflows that produce better results than ad-hoc answers.

Key routing rules:
- Product ideas, "is this worth building", brainstorming → invoke office-hours
- Bugs, errors, "why is this broken", 500 errors → invoke investigate
- Ship, deploy, push, create PR → invoke ship
- QA, test the site, find bugs → invoke qa
- Code review, check my diff → invoke review
- Update docs after shipping → invoke document-release
- Weekly retro → invoke retro
- Design system, brand → invoke design-consultation
- Visual audit, design polish → invoke design-review
- Architecture review → invoke plan-eng-review
