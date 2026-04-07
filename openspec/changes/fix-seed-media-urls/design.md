## Context

시드 데이터(`apps/api/db/seed.sql`)의 이미지 핀 7개가 가짜 로컬 경로(`image/seed-*.jpg`, `image/seed-*.png`)를 media_url로 사용하고 있다. 이 경로는 실제 파일이 존재하지 않아 프론트엔드에서 이미지 로드에 실패한다.

한편 같은 시드 데이터의 og_image 컬럼에는 이미 실제 접근 가능한 Unsplash URL이 들어 있다. 이 값을 media_url에 재활용하면 추가 외부 의존성 없이 문제를 해결할 수 있다.

## Goals / Non-Goals

**Goals:**
- 이미지 타입 핀의 media_url을 실제 로드 가능한 Unsplash URL로 교체
- 기존 og_image 값을 그대로 재활용하여 일관성 유지

**Non-Goals:**
- 오디오/비디오 핀의 media_url 변경 (카드 UI에 영향 없음)
- 프론트엔드 코드 변경
- 프로덕션 데이터 마이그레이션

## Decisions

### media_url 값 소스

**선택:** 기존 seed.sql의 og_image 컬럼에 있는 Unsplash URL을 media_url에 복사

각 이미지 핀의 매핑:

| Pin ID (끝 4자리) | 제목 | 현재 media_url | 변경할 media_url (= 기존 og_image) |
|---|---|---|---|
| 0003 | 밤의 정원 | `image/seed-night-garden.jpg` | `https://images.unsplash.com/photo-1579783902614-a3fb3927b6a5?w=400&h=500&fit=crop` |
| 0004 | 캐릭터 디자인 - 루나 | `image/seed-luna-design.jpg` | `https://images.unsplash.com/photo-1618005182384-a83a8bd57fbe?w=400&h=600&fit=crop` |
| 0005 | 앨범 자켓 - Dreamscape | `image/seed-album-jacket.jpg` | `https://images.unsplash.com/photo-1549490349-8643362247b5?w=400&h=350&fit=crop` |
| 0008 | Pixel Dungeon | `image/seed-pixel-dungeon.png` | `https://images.unsplash.com/photo-1550745165-9bc0b252726f?w=400&h=300&fit=crop` |
| 0009 | Sound Visualizer | `image/seed-sound-vis.png` | `https://images.unsplash.com/photo-1558618666-fcd25c85f82e?w=400&h=400&fit=crop` |
| 0010 | 잊혀진 계절 | `image/seed-forgotten-season.jpg` | og_image 없음 -- 새 Unsplash URL 할당 필요 |
| 0011 | 카페 루미에르 | `image/seed-cafe-lumiere.jpg` | og_image 없음 -- 새 Unsplash URL 할당 필요 |
| 0012 | Neon Rain 가사 | `image/seed-neon-rain-lyrics.jpg` | og_image 없음 -- 새 Unsplash URL 할당 필요 |

og_image가 NULL인 핀 3개(0010, 0011, 0012)는 콘텐츠 테마에 맞는 Unsplash URL을 새로 선정한다.

**대안:** picsum.photos 등 다른 placeholder 서비스
- 단점: 이미 og_image에 Unsplash URL이 있으므로 통일성이 떨어짐

## Risks / Trade-offs

- **외부 의존성**: Unsplash CDN이 다운되면 개발 환경에서도 이미지 로드 실패. 하지만 시드 데이터 용도이므로 허용 가능
- **롤백**: seed.sql 한 파일 변경이므로 git revert로 즉시 복구 가능
