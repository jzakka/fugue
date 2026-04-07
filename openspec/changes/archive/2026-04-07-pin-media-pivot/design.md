## Context

현재 Fugue의 핀 모델은 외부 URL이 핵심이다. 사용자가 URL을 입력하면 서버가 OG 메타데이터를 fetch하고, 그 정보로 카드를 구성한다. 하지만 제품 방향이 Pinterest 모델(미디어 중심 큐레이션)로 전환됨에 따라, 미디어 파일 자체가 핀의 본체가 되어야 한다.

기존 스택: Go(Chi) + PostgreSQL + Redis + Next.js. 파일 업로드 인프라는 현재 없음.

## Goals / Non-Goals

**Goals:**
- 핀 생성 시 미디어 파일(이미지/음원/비디오) 업로드를 필수로 만든다
- URL은 선택적 원본 출처 링크로 남긴다
- 분야(field) 필드를 제거하고, 미디어 타입 + 태그가 분류를 대체한다
- 사전 정의된 태그 시스템을 도입하여 추천 품질을 높인다
- 로컬 개발 환경에서 MinIO로, 프로덕션에서 S3로 파일을 저장한다

**Non-Goals:**
- 미디어 트랜스코딩/리사이징 (v1에서는 원본 저장, 추후 고려)
- 태그 자동 추천 (ML 기반, 추후 고려)
- 미디어 스트리밍 최적화 (CDN 세팅은 추후)
- 기존 핀 데이터 마이그레이션 (시드 데이터 재생성으로 대체)

## Decisions

### 1. 파일 스토리지: S3 호환 스토리지

**선택**: S3(prod) + MinIO(dev)

**대안 검토**:
- 로컬 파일시스템: 단순하지만 스케일링 불가, 컨테이너 재시작 시 유실
- Cloudflare R2: S3 호환이지만 egress 무료가 장점. 아직 설정 복잡

**이유**: Go의 `aws-sdk-go-v2`로 S3 API를 쓰면 MinIO와 100% 호환. 환경 변수로 endpoint만 바꾸면 됨. 인프라는 사용자가 Terraform으로 직접 구축 예정.

### 2. 업로드 방식: 서버 사이드 업로드 (v1)

**선택**: 클라이언트 → Go API (multipart) → S3

**대안 검토**:
- Presigned URL 직접 업로드: 서버 부하 적지만 CORS/보안 설정 복잡
- tus 프로토콜: 대용량 resumable 업로드. 오버엔지니어링.

**이유**: v1은 파일 크기 제한(이미지 10MB, 오디오 50MB, 비디오 100MB)이 있으니 서버 경유가 단순하고 충분. 추후 대용량 필요 시 presigned URL로 전환.

### 3. 태그 시스템: DB 테이블 + 시드 데이터

**선택**: `tags` 테이블 (id, name, category, slug) + `pin_tags` 조인 테이블

**대안 검토**:
- 기존 TEXT[] 배열 유지 + 프론트엔드에서 자동완성: 스키마 변경 최소화. 하지만 태그 일관성 보장 불가
- 외부 taxonomy 서비스: Pinterest처럼 Knowledge Graph 서비스. 오버엔지니어링

**이유**: 정규화된 태그 테이블이 중복 방지, 통계, 추천에 유리. 초기 태그는 시드 SQL로 투입. 카테고리별 그룹핑으로 UI 탐색 지원.

### 4. 태그 초기 데이터: 크리에이티브 분야 특화

Fugue는 음악/미술/영상/코드/글의 크로스미디어 플랫폼이므로, Pinterest의 범용 taxonomy와 달리 크리에이티브 분야에 특화된 태그를 설계한다.

카테고리 예시:
- 스타일: 사이버펑크, 판타지, 미니멀, 레트로, 몽환, ...
- 장르(음악): 힙합, 일렉트로닉, 재즈, 클래식, 앰비언트, ...
- 기법: 수채화, 3D모델링, 모션그래픽, 타이포그래피, ...
- 도구: Photoshop, Blender, Unity, Ableton, Premiere, ...
- 분위기: 따뜻한, 어두운, 밝은, 잔잔한, 강렬한, ...
- 용도: 앨범아트, 게임, 뮤직비디오, 포스터, 배경음, ...

초기 200~500개, 이후 운영하며 추가.

### 5. 태그 개수 제한: 최대 10개

기존 자유 입력 태그는 최대 5개였으나, 사전 정의 태그 시스템은 구조화된 카테고리별 태그이므로 표현력 확보를 위해 최대 10개로 확장. 최소 1개 필수는 유지 (태그 없는 핀은 추천/탐색 불가).

### 6. 미디어 업로드 모듈: `internal/storage/` 패키지

미디어 업로드 로직(S3 클라이언트, MIME 검증, 파일 크기 제한)은 `internal/storage/` 패키지로 분리. pin 핸들러에서 호출하되, 추후 fuguebot 등 다른 모듈에서도 재사용 가능.

### 7. 미디어 타입 감지: MIME 타입 기반

서버에서 업로드된 파일의 Content-Type을 검증:
- `image/*` → image
- `audio/*` → audio
- `video/*` → video

허용 포맷:
- 이미지: JPEG, PNG, GIF, WebP
- 오디오: MP3, WAV, OGG, FLAC
- 비디오: MP4, WebM

### 8. `og_image`/`og_data` vs `media_url` 역할 분담

`media_url`이 항상 존재하므로 PinCard 썸네일과 미디어 재생의 기본 소스는 `media_url`. `og_image`/`og_data`는 URL이 입력된 경우에만 존재하며, 핀 상세 페이지에서 원본 출처 정보(사이트명, 설명 등)를 보조 표시하는 용도로 유지.

### 9. `pin_count` 필드 제거

기존 `pin_count`는 "같은 URL을 핀한 유저 수"로, URL 기반 모델에서 의미가 있었다. 미디어 중심 모델에서는 URL이 선택이므로 이 지표의 의미가 사라진다. `pin_count` 컬럼과 `UpdatePinCountByURL` 쿼리를 제거한다. 추후 "좋아요" 등 별도 인터랙션 지표로 대체.

### 10. DB 스키마 변경

```sql
-- 새 테이블
CREATE TABLE tags (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(50) NOT NULL UNIQUE,
    slug VARCHAR(50) NOT NULL UNIQUE,
    category VARCHAR(30) NOT NULL,
    display_order INT DEFAULT 0
);

CREATE TABLE pin_tags (
    pin_id UUID NOT NULL REFERENCES pins(id) ON DELETE CASCADE,
    tag_id UUID NOT NULL REFERENCES tags(id),
    PRIMARY KEY (pin_id, tag_id)
);
CREATE INDEX idx_pin_tags_tag ON pin_tags(tag_id);

-- pins 테이블 변경
ALTER TABLE pins ADD COLUMN media_url VARCHAR(500) NOT NULL;
ALTER TABLE pins ADD COLUMN media_type VARCHAR(10) NOT NULL CHECK (media_type IN ('image', 'audio', 'video'));
ALTER TABLE pins ALTER COLUMN url DROP NOT NULL;  -- URL을 선택으로
ALTER TABLE pins DROP COLUMN field;               -- 분야 제거
ALTER TABLE pins DROP COLUMN tags;                -- TEXT[] 태그 제거 (pin_tags로 대체)
ALTER TABLE pins DROP COLUMN pin_count;           -- URL 기반 핀 카운트 제거 (Decision 9)
```

## Risks / Trade-offs

- **파일 크기 vs 서버 부하**: 서버 경유 업로드는 Go 서버 메모리를 소비. 파일 크기 제한 + `multipart.Reader`의 스트리밍 처리로 완화. → 미디어 크기 제한 적용 (이미지 10MB, 오디오 50MB, 비디오 100MB)
- **태그 확장성**: 초기 시드 태그가 부족할 수 있음. → 운영 중 관리자 태그 추가 기능은 추후 구현. v1은 시드 데이터 + DB 직접 INSERT로 관리
- **기존 데이터 호환**: 현재 핀 데이터는 URL 필수 + field 있음. → 시드 데이터를 새 스키마에 맞게 재생성. 프로덕션 데이터 없으므로 마이그레이션 불필요
- **OG fetch 기능 위상 변화**: URL이 선택이 되면 OG fetch의 역할이 줄어듦. → URL 입력 시에만 OG 프리뷰 표시하는 보조 기능으로 유지
- **S3 orphaned files**: 핀 삭제 시 S3 오브젝트는 즉시 삭제하지 않는다. v1에서는 orphaned file을 허용하고, S3 lifecycle policy(예: 90일 미참조 오브젝트 삭제)로 정리. 즉시 삭제는 삭제 실패 시 트랜잭션 롤백 복잡도가 높아 오버엔지니어링.
