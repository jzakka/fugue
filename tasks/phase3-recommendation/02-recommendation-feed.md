# 추천 기반 피드 API

**상태**: [ ] 미착수
**우선순위**: P0
**분류**: 신규
**의존**: 01-interactions, Phase 1 핀 CRD

## 엔드포인트

```
GET /api/feed  [auth, 비인증 시 최신순 fallback]
query: limit, cursor
```

## 추천 로직 (v1 태그 휴리스틱)

```
1. 유저의 핀에서 태그 빈도 집계 (상위 10개 태그)
2. 해당 태그를 가진 작품을 가중 점수순 정렬
3. 이미 핀한 작품 제외
4. 결과를 Redis에 캐싱 (TTL 5분)
```

## 피드 혼합 비율

- 핀 < 10개: 100% 최신순
- 핀 >= 10개: 50% 추천 + 50% 최신

## 할 일

- [ ] `internal/feed/` 패키지 생성
- [ ] 태그 빈도 집계 sqlc 쿼리
- [ ] 추천 후보 쿼리 (태그 가중 점수)
- [ ] Redis 캐시 로직
- [ ] handler: 인증 여부에 따라 추천/최신순 분기
- [ ] 커서 기반 페이지네이션
- [ ] main.go 라우트 등록
- [ ] 프론트: 피드 페이지를 추천 API로 교체

## 진화 로드맵

- v1 (현재): 태그 빈도 휴리스틱
- v2: 피처스토어 도입 (유저 피처 + 작품 피처 관리)
- v3: ML 모델 (피처스토어 기반 학습)

## 영향 범위

- `apps/api/internal/feed/` (신규)
- `apps/api/db/queries/feed.sql` (신규)
- `apps/api/cmd/server/main.go`
- `apps/web/src/app/page.tsx` (피드를 추천 API로 교체)
- `apps/web/src/components/feed/FeedContainer.tsx`
