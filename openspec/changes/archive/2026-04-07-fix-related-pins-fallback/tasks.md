## 1. Fallback SQL 쿼리 추가

- [x] 1.1 pins.sql에 FallbackRelatedByMediaType 쿼리 추가 — 같은 미디어 타입의 최신 핀을 반환하되, 제외할 핀 ID 목록을 받는다
- [x] 1.2 pins.sql에 FallbackRelatedLatest 쿼리 추가 — 미디어 타입 무관하게 최신 핀을 반환하되, 제외할 핀 ID 목록을 받는다
- [x] 1.3 sqlc generate 실행하여 Go 코드 재생성

## 2. API 핸들러 fallback 로직 구현

- [x] 2.1 PinQuerier 인터페이스에 새 쿼리 메서드 추가
- [x] 2.2 기존 handler_test.go의 mockQuerier에 새 메서드 stub 추가 (FallbackRelatedByMediaType, FallbackRelatedLatest)
- [x] 2.3 Related 핸들러에서 태그 0개일 때 early return 제거하고 fallback 흐름으로 전환
- [x] 2.4 Related 핸들러에 다단계 fallback 로직 구현 — 1단계(태그), 2단계(미디어타입), 3단계(최신) 순으로 10개 채우기
- [x] 2.5 중복 핀 ID 제외 로직 구현 — 이전 단계 결과의 핀 ID를 다음 단계 쿼리의 제외 목록에 포함

## 3. 프론트엔드 에러 처리 개선

- [x] 3.1 핀 상세 페이지의 연관 핀 API 호출 catch 블록에 console.error 추가

## 4. 테스트

- [x] 4.1 Related 핸들러 단위 테스트 — 태그 있는 핀의 정상 연관 핀 반환
- [x] 4.2 Related 핸들러 단위 테스트 — 태그 없는 핀의 fallback 연관 핀 반환
- [x] 4.3 Related 핸들러 단위 테스트 — 태그 매칭 부족 시 미디어 타입 fallback
- [x] 4.4 Related 핸들러 단위 테스트 — 중복 핀 제외 확인
