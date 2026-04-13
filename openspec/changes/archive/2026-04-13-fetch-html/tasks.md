## 1. HTTP 클라이언트 설정

- [x] 1.1 fetchHTML 메서드에 커스텀 Transport를 가진 http.Client 생성
- [x] 1.2 타임아웃을 10초로 설정
- [x] 1.3 최대 리다이렉트를 5번으로 설정
- [x] 1.4 User-Agent 헤더 "FugueBot/1.0 (+https://fugue.app)" 추가

## 2. fetchHTML에서 요청 실행

- [x] 2.1 apps/api/internal/bot/pioneer.go의 fetchHTML 스텁 교체 (251-255줄)
- [x] 2.2 GET 메서드와 context로 http.Request 생성
- [x] 2.3 작업 1에서 설정한 클라이언트로 요청 실행
- [x] 2.4 요청 실행 에러 처리 (네트워크, DNS, 타임아웃)

## 3. 응답 처리

- [x] 3.1 HTTP 상태 코드 확인, 2xx 아니면 에러 반환
- [x] 3.2 io.ReadAll로 응답 본문 읽기
- [x] 3.3 defer로 응답 본문 닫기
- [x] 3.4 응답 본문이 비어있지 않은지 검증

## 4. 에러 메시지

- [x] 4.1 타임아웃 에러를 설명적인 메시지로 감싸기
- [x] 4.2 4xx/5xx 응답에 대해 HTTP 상태 코드를 에러에 포함
- [x] 4.3 빈 응답 본문에 대해 명확한 에러 반환

## 5. 테스트

- [x] 5.1 실제 URL (unsplash.com)로 fetchHTML 테스트
- [x] 5.2 Pioneer가 이제 그래프 노드를 생성하는지 확인
- [x] 5.3 느린 엔드포인트로 타임아웃 처리 테스트
- [x] 5.4 404 URL로 에러 처리 테스트

## 6. Pioneer 에러 로깅

- [x] 6.1 pioneer.go의 fetchHTML 에러 처리 부분에 로그 출력 추가 (100-102줄)
- [x] 6.2 에러 로그에 URL과 에러 메시지 포함
