# API 엔드포인트

## Auth (구현 완료)

```
GET    /api/auth/providers                        공개 프로바이더 목록
GET    /api/auth/{provider}/login                 OAuth 로그인 시작
GET    /api/auth/{provider}/callback              OAuth 콜백
POST   /api/auth/refresh                          토큰 갱신
POST   /api/auth/logout                           로그아웃
GET    /api/auth/me                    [auth]      현재 유저 정보
```

## Creator (프로필)

```
GET    /api/creators/{id}                          유저 공개 프로필 (닉네임, 아바타)
GET    /api/creators/me                [auth]      내 프로필
PUT    /api/creators/me                [auth]      프로필 수정 (닉네임, 아바타만)
```

## Pin (핀)

```
POST   /api/pins                       [auth, rate: 30/min/user]
       content-type: multipart/form-data
       fields: media (file, required), title, description, url (optional),
               tag_ids (uuid[], required), og_image (optional)

GET    /api/pins/{id}                              핀 상세 (+ creator 정보)

DELETE /api/pins/{id}                  [auth]      핀 삭제 (본인만)

GET    /api/pins                                   핀 목록 (미디어타입/태그 필터, 페이지네이션)
       query: media_type, tag_ids, limit, offset, creator_id

GET    /api/pins/{id}/related                      연관 핀 (태그 기반, 최대 10개)

GET    /api/pins/{id}/boards                       핀 소속 공개 보드 목록 (최대 10개)
```

## Tag (태그)

```
GET    /api/tags                                   사전정의 태그 목록
       query: category, q
GET    /api/tags/popular                           인기 태그 목록
       query: limit (기본 20, 최대 50)
```

## Search (검색)

```
GET    /api/search                                 통합 검색
       query: q, type, tag_ids, limit, offset
```

## Board (보드)

```
POST   /api/boards                     [auth]      보드 생성
       body: { name, description?, is_public? }

GET    /api/boards/{id}                             보드 조회 (공개 또는 본인)
       query: limit, offset (핀 목록 페이지네이션)
       response: { board, pins, has_more }

PUT    /api/boards/{id}                [auth]      보드 수정 (소유자만)
       body: { name?, description?, is_public? }

DELETE /api/boards/{id}                [auth]      보드 삭제 (소유자만)

GET    /api/boards                                  보드 목록
       query: creator_id (본인이면 전체, 타인이면 공개만)

POST   /api/boards/{id}/pins          [auth]      보드에 핀 추가 (소유자만)
       body: { pin_id }

DELETE /api/boards/{id}/pins/{pin_id}  [auth]      보드에서 핀 제거 (소유자만)
```

## Feed (추천)

```
GET    /api/feed                                    추천 기반 피드
       query: limit, cursor
       인증 시: 개인화 추천, 비인증 시: 최신순 fallback
```

## Interaction (행동 기록)

```
POST   /api/interactions               [auth]      행동 기록
       body: { pin_id, type }
       type: 'view' | 'pin' | 'board_add'
```

## OG Metadata

```
POST   /api/og/fetch                   [rate: 20/min/IP]
       body: { url }
       response: { title, description, image, site_name, url, detected_field }
```

## 범례

- `[auth]` = JWT 인증 필요 (auth.JWTMiddleware)
- `[rate: N/min/X]` = Redis 기반 rate limit
