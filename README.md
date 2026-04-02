# Fugue

크로스미디어 창작물 큐레이션 플랫폼. Pinterest가 이미지만이라면, Fugue는 음악, 일러스트, 영상, 글, 코드를 분야를 넘나들며 한곳에서 발견한다.

마스코트: 헤드셋 끼고 붓 든 복어 (Fugue ≈ Fugu)

## 기술 스택

| 계층 | 기술 |
|------|------|
| Frontend | Next.js 15 (App Router), TypeScript |
| Backend | Go + Chi router |
| ORM | sqlc |
| DB | PostgreSQL 16 |
| Cache | Redis |
| Auth | OAuth 2.0 (Google, Discord) |
| Infra | Terraform, EKS, ArgoCD, Helm |

## MVP 기능

- **핀**: 외부 창작물 URL → OG 자동 프리뷰 → 분야/태그 선택 → 큐레이션
- **보드**: 핀을 주제별로 묶는 컬렉션
- **추천 피드**: 태그 기반 취향 학습 → 개인화된 작품 추천
- **연관 작품**: 작품 상세에서 유사 작품 자동 표시
- **소셜 로그인**: Google, Discord OAuth (구현 완료)

## 구현 현황

- [x] OAuth 소셜 로그인 (Google, Discord)
- [x] 유저 프로필 (닉네임, 아바타)
- [x] 작품 피드 (분야/태그 필터, 페이지네이션)
- [ ] 핀 생성/삭제 + OG fetch
- [ ] 보드 CRUD
- [ ] 추천 피드
- [ ] 암묵적 취향 학습
- [ ] 연관 작품

## 문서

- [PRD](docs/ko/PRD.md)
- [MVP 기능 스펙](docs/mvp-features.md)
- [API 엔드포인트](docs/api-endpoints.md)
- [ERD](docs/erd.md)
- [아키텍처](docs/architecture.md)
- [기술 스택 상세](docs/tech-stack.md)

## 로컬 실행

```bash
docker-compose up -d          # PostgreSQL + Redis
cd apps/api && go run cmd/server/main.go
cd apps/web && npm run dev
```
