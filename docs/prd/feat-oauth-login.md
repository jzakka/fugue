## Product Requirements Document: OAuth 소셜 로그인

**Author**: chungsanghwa
**Date**: 2026-03-31
**Status**: Draft
**Stakeholders**: -

### 1. Executive Summary

Fugue 플랫폼의 소셜 로그인 기능. Google, Discord, Twitter OAuth 2.0을 통해 크리에이터가 별도 회원가입 없이 기존 소셜 계정으로 로그인하고, 자동으로 크리에이터 프로필이 생성되도록 한다. 현재 개발 환경에서 사용 중인 auth stub(`X-Creator-ID` 헤더)을 실제 OAuth로 교체하는 작업이다.

### 2. Background & Context

- Fugue는 창작물 기반 협업 매칭 플랫폼으로, 타겟 유저는 음악/일러스트/영상 등 창작자
- 창작자 커뮤니티는 Twitter, Discord 활동이 활발하며, Google은 범용 접근성 확보 용도
- 현재 프로젝트 스캐폴드 단계에서 `X-Creator-ID` 헤더 기반 auth stub이 설계되어 있음 (openspec/project-init)
- 미들웨어 인터페이스가 auth stub 교체를 전제로 설계됨 — 핸들러 변경 없이 교체 가능
- AGENTS.md에 "같은 이메일이면 자동 병합" 정책이 명시되어 있음

### 3. Objectives & Success Metrics

**Goals**:
1. Google, Discord, Twitter 3개 provider로 소셜 로그인/회원가입 가능
2. 최초 로그인 시 크리에이터 프로필 자동 생성 (닉네임, 아바타 등 OAuth 프로필에서 추출)
3. 같은 이메일의 다른 provider로 로그인 시 계정 자동 병합
4. JWT 기반 인증으로 httpOnly 쿠키 + Authorization 헤더 모두 지원
5. auth stub을 실제 OAuth 미들웨어로 교체

**Non-Goals**:
1. 이메일/비밀번호 로그인 — 소셜 로그인만 지원
2. 관리자 권한/역할 기반 접근 제어 (RBAC) — MVP에서 불필요
3. 2FA/MFA — MVP 범위 밖
4. OAuth provider 관리 화면 (연결/해제 UI) — 향후 프로필 설정에서 처리

**Success Metrics**:

| Metric | Current | Target | Measurement |
|--------|---------|--------|-------------|
| 로그인 성공률 | N/A (stub) | >= 95% | OAuth callback 성공/실패 비율 |
| 로그인 → 프로필 완성 전환율 | N/A | >= 80% | 최초 로그인 후 프로필 편집 완료 비율 |
| 계정 병합 정확도 | N/A | 100% | 같은 이메일 다른 provider 로그인 시 기존 계정에 연결 |

### 4. Target Users & Segments

| 세그먼트 | 설명 | 주요 OAuth provider |
|----------|------|---------------------|
| 음악 크리에이터 | 작곡, 사운드디자인, 작사 | Twitter, Discord |
| 비주얼 크리에이터 | 일러스트, 영상편집, 3D모델링 | Twitter, Discord |
| 일반 접근 유저 | 탐색/추천만 사용하는 유저 | Google |

창작자 커뮤니티 특성상 Twitter/Discord가 주력이며, Google은 진입장벽을 낮추는 보조 수단.

### 5. User Stories & Requirements

**P0 — Must Have**:

| # | User Story | Acceptance Criteria |
|---|-----------|-------------------|
| 1 | 유저는 Google 계정으로 로그인할 수 있다 | Google OAuth 2.0 flow 완료 → JWT 발급 → 인증 상태 유지 |
| 2 | 유저는 Discord 계정으로 로그인할 수 있다 | Discord OAuth 2.0 flow 완료 → JWT 발급 → 인증 상태 유지 |
| 3 | 유저는 Twitter 계정으로 로그인할 수 있다 | Twitter OAuth 2.0 flow 완료 → JWT 발급 → 인증 상태 유지 |
| 4 | 최초 로그인 시 크리에이터 프로필이 자동 생성된다 | OAuth 프로필에서 닉네임/아바타 추출 → creators 테이블에 INSERT |
| 5 | 같은 이메일의 다른 provider 로그인 시 기존 계정에 연결된다 | email 기준으로 기존 creator 조회 → oauth_accounts에 새 provider 추가 |
| 6 | JWT는 httpOnly 쿠키와 Authorization 헤더 모두 지원한다 | 쿠키 자동 첨부 (웹) + Bearer 토큰 (API 클라이언트) 모두 인증 통과 |
| 7 | 유저는 로그아웃할 수 있다 | `POST /api/auth/logout` → refresh token 무효화, 쿠키 삭제 |
| 8 | auth stub 미들웨어가 OAuth 미들웨어로 교체된다 | 기존 핸들러 코드 변경 없이 미들웨어만 교체 |

**P1 — Should Have**:

| # | User Story | Acceptance Criteria |
|---|-----------|-------------------|
| 9 | Access token 만료 시 refresh token으로 자동 갱신된다 | refresh token으로 `POST /api/auth/refresh` → 새 access token 발급 |
| 10 | 로그인 페이지에 3개 provider 버튼이 표시된다 | `/login` 페이지에 Google, Discord, Twitter 로그인 버튼 렌더링 |
| 11 | OAuth 실패 시 사용자에게 에러 메시지가 표시된다 | provider 에러, 사용자 취소 등 → 로그인 페이지에 에러 메시지 표시 |
| 12 | 미인증 상태에서 보호된 페이지 접근 시 로그인으로 리다이렉트된다 | 인증 필요 페이지 접근 → `/login?redirect={원래 URL}` |

**P2 — Nice to Have / Future**:

| # | User Story | Acceptance Criteria |
|---|-----------|-------------------|
| 13 | 프로필 설정에서 연결된 소셜 계정을 확인할 수 있다 | 연결된 provider 목록 표시 |
| 14 | 추가 소셜 계정을 연결/해제할 수 있다 | 프로필 설정에서 provider 추가 연결/해제 |
| 15 | 로그인 이력을 확인할 수 있다 | 최근 로그인 시간, provider, IP 표시 |

### 6. Solution Overview

**인증 흐름**:
```
[유저] → 로그인 버튼 클릭
  → [Frontend] /api/auth/{provider}/login 리다이렉트
  → [Backend] OAuth provider 인증 페이지로 리다이렉트
  → [Provider] 유저 동의 → callback URL로 리다이렉트
  → [Backend] POST /api/auth/{provider}/callback
    → authorization code → access token 교환
    → provider에서 유저 프로필 조회
    → email로 기존 creators 조회
      → 있으면: oauth_accounts에 provider 추가
      → 없으면: creators + oauth_accounts INSERT
    → JWT (access + refresh) 발급
    → httpOnly 쿠키 설정 + response body에 access token
  → [Frontend] 홈 또는 원래 페이지로 리다이렉트
```

**JWT 전략**:
- Access token: 15분 TTL, 짧게 유지
- Refresh token: 7일 TTL, Redis에 저장하여 무효화 가능
- httpOnly + Secure + SameSite=Lax 쿠키로 웹 자동 첨부
- Authorization: Bearer 헤더로 API 클라이언트 지원

**DB 변경사항**:

기존 `creators` 테이블에 `email` 컬럼 추가, `oauth_accounts` 테이블 신규 생성:

```sql
-- creators 테이블에 email 추가
ALTER TABLE creators ADD COLUMN email VARCHAR(320) UNIQUE;

-- OAuth 계정 테이블
CREATE TABLE oauth_accounts (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    creator_id  UUID NOT NULL REFERENCES creators(id) ON DELETE CASCADE,
    provider    VARCHAR(20) NOT NULL,  -- 'google', 'discord', 'twitter'
    provider_id VARCHAR(200) NOT NULL, -- provider 측 유저 ID
    email       VARCHAR(320),
    profile     JSONB,                 -- provider 원본 프로필 데이터
    created_at  TIMESTAMPTZ DEFAULT now(),
    UNIQUE(provider, provider_id)
);

CREATE INDEX idx_oauth_accounts_creator ON oauth_accounts(creator_id);

-- Refresh token 저장 (Redis 사용, 테이블 불필요)
```

**기술 구현**:
- Backend: Go + Chi 미들웨어에서 JWT 검증, `golang.org/x/oauth2` 패키지 사용
- Frontend: Next.js 미들웨어에서 인증 상태 확인, 리다이렉트 처리
- auth stub의 미들웨어 인터페이스를 그대로 활용하여 교체

### 7. Open Questions

| Question | Owner | Deadline |
|----------|-------|----------|
| Twitter OAuth 2.0 앱 승인에 얼마나 걸리는가? (지연 시 Discord/Google 먼저 구현) | chungsanghwa | 구현 착수 전 |
| OAuth 콜백 URL 도메인 — 개발/스테이징/프로덕션 환경별 설정 방식 | chungsanghwa | 인프라 셋업 시 |
| 계정 병합 시 이메일이 없는 provider (Twitter는 이메일 미제공 가능) 처리 방식 | chungsanghwa | 구현 시 |

### 8. Timeline & Phasing

타임라인 미확정. 아래는 구현 순서 제안:

**Phase 1: 코어 인증**
- DB 마이그레이션 (oauth_accounts, creators.email)
- JWT 발급/검증 미들웨어
- Google OAuth 구현 (가장 문서화 잘 되어 있고 승인 빠름)
- 로그인/로그아웃 API
- auth stub 교체

**Phase 2: 추가 provider + 프론트엔드**
- Discord OAuth 구현
- Twitter OAuth 구현
- 로그인 페이지 UI
- 인증 리다이렉트 미들웨어 (Next.js)
- refresh token 자동 갱신

**Phase 3: 계정 병합 + 안정화**
- 이메일 기반 계정 자동 병합
- OAuth 에러 핸들링
- 엣지 케이스 처리 (이메일 없는 Twitter 등)
