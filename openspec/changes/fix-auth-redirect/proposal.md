## Why
모든 서브 라우트(/pins/{id}, /search, /creators/{id} 등)가 비로그인 상태에서 로그인 페이지로 강제 리다이렉트된다. 핀 상세, 검색 결과, 크리에이터 프로필은 공개 페이지로 인증 없이 접근 가능해야 한다. API는 정상 응답하므로 프론트엔드 라우팅/인증 미들웨어 설정 문제.

## What Changes
- Next.js 미들웨어 또는 라우팅 설정에서 공개 페이지 경로를 인증 예외로 설정
- 인증이 필요한 경로만 명시적으로 보호 (/mypage, /pin/new 등)

## Capabilities
### New Capabilities
_(없음)_
### Modified Capabilities
_(없음 — 기존 동작 복원)_

## Impact
- Frontend: Next.js 미들웨어/라우팅 설정 수정
- 인증 필요 경로: /mypage, /pin/new
- 공개 경로: /, /pins/{id}, /search, /creators/{id}, /boards/{id}, /login
