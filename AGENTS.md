# Fugue - AGENTS.md

## 언어

모든 응답은 한국어로 작성한다.

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
| 클래스/함수명 | `OGService.Fetch`, `PinQuerier` | "OG 메타데이터 조회 서비스", "핀 쿼리 인터페이스" |
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
| `bot` | 외부 콘텐츠 크롤러 (크롤 엔진, Source 플러그인, 미디어 다운로드, 크롤 관리) |

**새 도메인 생성 기준** — 다음 조건을 모두 충족할 때만:
1. 위 도메인 어디에도 행위가 포함되지 않는다
2. 독립된 엔티티 또는 바운디드 컨텍스트를 형성한다
3. 최소 3개 이상의 독립 요구사항이 예상된다

## Makefile 타겟

모든 개발 명령어는 루트 `Makefile`에 정의되어 있다. `apps/api/Makefile`은 폐기됨.

### 개발 환경

```bash
make dev           # 전체 스택 실행 (인프라 + 마이그레이션 + API + Web)
make dev-stop      # 전체 종료
make dev-infra     # PostgreSQL + Redis만 실행
make dev-api       # API 서버만 실행
make dev-web       # Next.js만 실행
```

### DB 마이그레이션

```bash
make migrate         # 마이그레이션 실행 (dev에 포함됨)
make migrate-up      # 마이그레이션 up
make migrate-down    # 마이그레이션 rollback
make migrate-create  # 새 마이그레이션 생성 (대화형)
make seed            # 시드 데이터 삽입
```

### 코드 품질

```bash
make lint          # golangci-lint 실행
make fmt           # goimports 포맷팅
make test          # Go + Frontend 테스트
```

### Bot 크롤러

```bash
make pioneer SITE=unsplash        # Pioneer 크롤러 실행 (SITE: unsplash|fma|pixiv)
make harvester                    # Harvester 워커 실행 (전 사이트 URL을 우선순위 순으로 소비)
make show-map                     # Bot 그래프 시각화 (graph.html 생성)
make fuguebot-progress            # Fuguebot 진행 현황/의존성 그래프 (fuguebot_graph.py)
```

#### Pioneer/Harvester 작업 시 필수 업데이트

Pioneer 또는 Harvester 관련 작업(신규 source 추가, 필터/스케줄러 변경, 크롤 엔진 수정 등)을 수행했다면 `fuguebot_graph.py`가 생성하는 fuguebot-graph에 표시되는 내용도 함께 업데이트해야 한다. 노드/엣지/상태 라벨 등이 실제 구현과 일치하도록 `make fuguebot-progress`로 확인한다.

#### Pioneer 동작 모델 (scheduler consumer)

Pioneer는 `URLScheduler`의 consumer이자 fanout B의 producer다. 한 루프 반복은 다음 순서로 동작한다: **`Dequeue(QueuePioneer)` → fetch → snapshot 저장 → link 추출 → `FilterChain.Apply` → `Enqueue(QueuePioneer, filteredURLs...)` + `EnqueueHarvester(url, snapshotKey)` → `SetStatus(url, "fetched", nil)`**. 인메모리 큐/visited 맵은 보유하지 않으며, URL 중복·우선순위·host 배려는 `URLScheduler`가 `pioneer_frontier` 테이블과 host token bucket으로 처리한다. 관련 스펙: `openspec/specs/pioneer/spec.md`.

#### Pioneer 링크 필터 정책

Pioneer가 Enqueue하기 전 적용하는 FilterChain 순서는 고정이다: **Domain → Extension → PathPattern → Robots → Dedup**. 각 필터의 책임은 다음과 같다.

- **DomainFilter (교차 사이트 기본 허용)**: `AllowKeywords`/`DenyKeywords` 두 리스트로 링크 호스트를 substring 매칭한다. 매칭 전 호스트는 lowercase + `www.` 제거로 정규화된다.
  - `AllowKeywords`가 비어 있으면 Deny에 걸리지 않은 모든 호스트를 통과시킨다 (Fugue의 크로스미디어 비전 기본값).
  - `DenyKeywords` 매칭이 `AllowKeywords`보다 우선한다.
  - 국가별 TLD에 대한 특별 처리는 없다. 필요한 키워드는 명시적으로 리스트에 추가한다.
- **RobotsFilter**: 호스트별 robots.txt를 최초 접근 시 lazy fetch하여 in-memory 캐시에 저장한다. User-agent는 `FugueBot` 우선, 없으면 `*` fallback (두 블록은 병합하지 않는다).
  - 캐시 TTL은 24시간이며, fetch 실패(네트워크 오류·타임아웃·5xx) 시 **fail-open**(모두 통과)으로 동작한다. 실패 상태도 같은 TTL로 캐시해 재시도 폭주를 막는다. 캐시는 in-memory per-process이므로 Pioneer 프로세스가 종료되면 비워지며, 여러 Pioneer 인스턴스는 캐시를 공유하지 않는다.
  - `Disallow` 규칙은 prefix 매칭으로 해당 경로를 차단한다. `Crawl-delay: N`가 명시되면 `scheduler-host-token-bucket`의 `SetHostRate(host, 1/N, 1)`을 호출해 해당 호스트의 토큰 버킷 rate를 갱신한다 (캐시 갱신 시점에만 1회).
- **Redirect chain**: 필터 체인은 `fetchHTML`이 반환하는 **최종 URL**에만 적용된다. 중간 redirect URL은 검사하지 않는다.

관련 스펙: `openspec/specs/bot/spec.md`의 DomainFilter / canonicalURL / RobotsFilter / 필터 체인 요구사항.

### 초기 설정

```bash
make setup         # Lefthook git hooks 설치
```

## 워크플로우 규칙

- 커밋 전에 반드시 `/codex` review를 실행할 것
- Makefile 타겟은 루트에만 추가한다 (`apps/api/Makefile`에 추가하지 말 것)
- **PR을 절대 생성하지 않는다.** `gh pr create`, GitHub UI 등 어떤 방식으로도 PR을 만들지 말 것. 변경사항은 항상 `main` 브랜치에 직접 커밋하고 `git push`로 `origin/main`에 머지한다 (로컬 커밋만으로는 push 완료가 아니다).
  - 사용자가 "ship", "deploy", "push", "PR 만들어" 등으로 요청해도 PR을 만들지 말고 main에 직접 push한다.
  - `/ship` 같은 스킬이 PR 생성을 시도하면 중단하고 main 직접 push로 대체한다.

## 배포 정책

로컬 개발만 하므로 상용에서 카나리 배포, 무중단 같은건 고려 안해도 된다.
