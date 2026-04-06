## 1. 스토리지 인프라

- [ ] 1.1 docker-compose에 MinIO 서비스 추가 + 기본 버킷 생성
- [ ] 1.2 Go API에 S3 클라이언트 모듈 추가 (aws-sdk-go-v2, 환경변수로 endpoint 전환)
- [ ] 1.3 미디어 업로드 핸들러 구현 (multipart 수신 → MIME 검증 → S3 저장 → URL 반환)

## 2. DB 마이그레이션

- [ ] 2.1 tags 테이블 + pin_tags 조인 테이블 생성 마이그레이션
- [ ] 2.2 pins 테이블 변경 마이그레이션 (media_url, media_type 추가 / url NOT NULL 해제 / field, tags 컬럼 제거)
- [ ] 2.3 태그 시드 데이터 작성 (카테고리별 200~500개)
- [ ] 2.4 핀 시드 데이터를 새 스키마에 맞게 재작성

## 3. 태그 API

- [ ] 3.1 태그 SQL 쿼리 작성 (전체 목록, 카테고리별, 검색) + sqlc 코드 생성
- [ ] 3.2 태그 핸들러 구현 (GET /api/tags, GET /api/tags?category=, GET /api/tags?q=)
- [ ] 3.3 라우트 등록 (main.go)

## 4. 핀 API 변경

- [ ] 4.1 핀 SQL 쿼리 변경 (CreatePin에 media_url/media_type 추가, field 제거, pin_tags 조인) + sqlc 코드 생성
- [ ] 4.2 핀 생성 핸들러 변경 (multipart 폼 파싱, 미디어 업로드 연동, 태그 ID 검증 + 연결)
- [ ] 4.3 핀 목록/상세 쿼리 변경 (field 필터 → media_type 필터, 태그를 pin_tags 조인으로)
- [ ] 4.4 피드 핸들러 변경 (분야 기반 추천 → 태그/미디어타입 기반 추천)
- [ ] 4.5 연관 핀 쿼리 변경 (field 우선 → media_type 우선)

## 5. 프론트엔드 변경

- [ ] 5.1 핀 생성 폼 재설계 (파일 업로드 UI + 미디어 프리뷰 + URL 선택 입력)
- [ ] 5.2 태그 선택 UI 구현 (카테고리별 그룹, 검색, 선택/해제)
- [ ] 5.3 PinCard 컴포넌트에 미디어타입별 프리뷰 (이미지: 썸네일, 오디오: 파형/플레이어, 비디오: 썸네일+재생)
- [ ] 5.4 핀 상세 페이지에 미디어 플레이어 추가
- [ ] 5.5 메인 화면 버튼 문구 "핀 생성"으로 변경, 분야 필터 → 미디어타입 필터로 변경
- [ ] 5.6 api.ts 타입/함수 업데이트 (Pin 타입, createPin을 FormData로, 태그 API 추가)

## 6. 테스트 + 정리

- [ ] 6.1 핀 핸들러 테스트 업데이트 (미디어 필수, URL 선택, 태그 ID 검증)
- [ ] 6.2 기존 openspec/specs/ 도메인 스펙 업데이트 (pin, feed 스펙에 변경사항 반영)
- [ ] 6.3 docs/ 문서 업데이트 (mvp-features.md, api-endpoints.md, erd.md)
