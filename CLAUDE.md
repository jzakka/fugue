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
│   │   └── internal/
│   │       ├── bot/       # Pioneer/Harvester 크롤러 (URLScheduler consumer)
│   │       ├── auth/      # 소셜 로그인
│   │       ├── pin/       # 핀 생성/조회
│   │       ├── board/     # 보드 관리
│   │       └── feed/      # 추천 피드
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

### 의존성

- **ffprobe/ffmpeg**: 비디오 업로드 시 서버 사이드 duration 검증에 필요
  - macOS: `brew install ffmpeg`
  - Docker: `apps/api/Dockerfile`에 포함됨
- **Graphviz** (선택): Bot graph PNG/SVG export 시 필요
  - macOS: `brew install graphviz`

### 로컬 실행

```bash
docker-compose up -d     # PostgreSQL + Redis
cd apps/api && go run cmd/server/main.go
cd apps/web && npm run dev
```

### Bot Graph Visualization

Pioneer가 크롤한 노드 그래프를 시각화:

```bash
make show-map  # 인터랙티브 HTML 생성
```

Harvester script 존재 표시:
- DB `bot_sources` 테이블에서 (site, node_type) 키로 조회하여 판정
- 존재하면 초록색(구현됨), 없으면 회색(미구현)으로 표시

### Fuguebot 진행 현황

OpenSpec 체인지 진행 현황 및 의존성 그래프 시각화:

```bash
make fuguebot-progress  # 진행 상황 + 의존성 그래프 HTML 생성 (브라우저 자동 오픈)
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
- OpenSpec 설계 리뷰 → invoke openspec-review
- OpenSpec 구현 리뷰 → invoke openspec-impl-review
