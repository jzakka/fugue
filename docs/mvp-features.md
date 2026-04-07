# MVP 기능 스펙

## 1. 핀 (Pin)

미디어 파일을 직접 업로드하여 큐레이션하는 핵심 단위.

- 미디어 파일 직접 업로드 (image/audio/video). multipart/form-data로 전송
- URL은 선택 항목 (원본 출처 표기용)
- OG 이미지: 선택적으로 og_image URL 첨부 가능
- 태그: 사전정의된 태그 목록에서 선택 (tag_ids, 필수). /api/tags로 목록 조회
- URL 유니크 제약 없음 (여러 사용자가 같은 작품 핀 가능)
- Create, Read, Delete만. Update 없음 (삭제 후 재핀)
- 소유권 주장이 아닌 큐레이션

## 2. 보드 (Board)

핀을 주제별로 묶는 컬렉션.

- 보드 이름 (필수) + 설명 (선택)
- 보드에 핀 추가/제거
- 공개/비공개 설정 (비공개 = 본인만 접근)
- 한 핀은 여러 보드에 속할 수 있음 (N:M)
- 프로필 = 내 보드 목록 + 전체 핀 목록

## 3. 추천 기반 피드

취향 기반 작품 추천. 단순 최신순이 아닌 개인화된 피드.

- v1: 태그 빈도 기반 휴리스틱 (내 핀의 태그 분포 → 같은 태그 작품 추천)
- 콜드 스타트: 핀 10개 미만이면 100% 최신순, 10개 이상이면 50% 추천 + 50% 최신
- 추천은 배치/Redis 캐싱 (요청마다 계산하지 않음)
- 진화 로드맵: 휴리스틱 → 피처스토어 → ML

## 4. 암묵적 취향 학습

클릭/조회 행동을 기록하여 추천에 활용.

- 이벤트 타입: view (작품 상세 조회), pin (핀 생성), board_add (보드에 핀 추가)
- Go API에서 비동기(channel + worker)로 Kinesis Firehose → S3에 Parquet 형식으로 적재
- 이벤트 스키마: event_id, user_id, pin_id, event_type, timestamp, context(JSON)
- S3 날짜 파티셔닝 (year/month/day/hour), Athena로 ad-hoc 분석
- v1에서는 추천 보조 시그널, 추후 ML 학습 데이터로 활용

## 5. 연관 핀

핀 상세 페이지 하단에 유사 핀 표시.

- 같은 태그를 가진 핀 중 태그 일치순 정렬
- 같은 media_type 우선, 크로스 미디어타입도 표시
- 최대 10개

## 6. 인증 (구현 완료)

- 소셜 로그인: Google OAuth, Discord OAuth
- JWT 기반 인증 (access + refresh token)
- 계정 병합: 같은 이메일이면 자동 병합

## 7. 프로필

- 닉네임 + 아바타 (OAuth에서 가져오거나 기본 아바타)
- 내 보드 목록 + 전체 핀 목록
- 포트폴리오 기능 없음 (역할 태그, 자기소개, SNS 연락처 없음)

## 8. Fuguebot (콘텐츠 크롤러)

외부 창작물 플랫폼을 주기적으로 크롤링하여 피드를 자동으로 채우는 봇.
Pinterestbot과 동일한 역할. 콜드 스타트 문제를 엔지니어링으로 해결.

- Colly 기반 크롤 엔진. robots.txt 존중, rate limit, User-Agent: Fuguebot/1.0
- 플랫폼별 Source 플러그인 아키텍처 (Go interface)
- MVP 플러그인 2개 (구조가 다른 2개 플랫폼)
- 외부 미디어(이미지/음원/비디오) 다운로드 → S3 미디어 버킷에 저장
- 수집한 콘텐츠로 핀 자동 생성 (creator_id = fuguebot 시스템 계정)
- URL 중복 체크 (이미 핀된 URL은 skip)
- OG 텍스트에서 사전정의 태그 자동 추출
- 크롤 통계를 S3 이벤트 파이프라인(Firehose)으로 로깅
- 크롤 상태 대시보드 API (관리자용)
- 크롤 소스 설정 API (동적으로 플랫폼 추가/제거)
- API 서버와 별도 바이너리 (cmd/bot/). K8s CronJob으로 실행
