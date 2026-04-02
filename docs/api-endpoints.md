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

## Pin (작품)

```
POST   /api/works                      [auth]      핀 생성
       body: { url, title, description, field, tags, og_image, og_data }

GET    /api/works/{id}                              핀 상세 (+ creator 정보)

DELETE /api/works/{id}                 [auth]      핀 삭제 (본인만)

GET    /api/works                                   피드 (분야/태그 필터, 페이지네이션)
       query: field, tags, limit, offset, creator_id

GET    /api/works/{id}/related                      연관 작품 (태그 기반, 최대 10개)
```

## Board

```
POST   /api/boards                     [auth]      보드 생성
       body: { name, description?, is_public? }

GET    /api/boards/{id}                             보드 조회 (공개 또는 본인)

PUT    /api/boards/{id}                [auth]      보드 수정 (소유자만)
       body: { name?, description?, is_public? }

DELETE /api/boards/{id}                [auth]      보드 삭제 (소유자만)

GET    /api/boards                                  보드 목록
       query: creator_id (본인이면 전체, 타인이면 공개만)

POST   /api/boards/{id}/pins          [auth]      보드에 핀 추가 (소유자만)
       body: { work_id }

DELETE /api/boards/{id}/pins/{work_id} [auth]      보드에서 핀 제거 (소유자만)
```

## Feed (추천)

```
GET    /api/feed                       [auth]      추천 기반 피드
       query: limit, cursor
       비인증 시: 최신순 fallback
```

## Interaction

```
POST   /api/interactions               [auth]      행동 기록
       body: { work_id, type }
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
- `[rate: N/min/IP]` = Redis 기반 IP별 rate limit
