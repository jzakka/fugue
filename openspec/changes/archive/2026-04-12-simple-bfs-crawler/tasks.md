## 1. 패키지 구조 및 인터페이스 정의

- [x] 1.1 `apps/api/internal/bot/crawler/` 패키지 디렉토리 생성
- [x] 1.2 `Fetcher` 인터페이스 정의 (Fetch 메서드, FetchResult 구조체)
- [x] 1.3 `Crawler` 인터페이스 정의 (Crawl 메서드)
- [x] 1.4 `Result`, `VisitedURL` 구조체 정의
- [x] 1.5 `BFSCrawler` 구조체 및 생성자 함수 구현 (Fetcher를 의존성으로 받음)

## 2. Fetcher 구현

- [x] 2.1 `FileFetcher` 구조체 및 생성자 구현
- [x] 2.2 `FileFetcher.Fetch()` 메서드 구현 (URL → 파일 경로 변환)
- [x] 2.3 `HTTPFetcher` 구조체 및 생성자 구현
- [x] 2.4 `HTTPFetcher.Fetch()` 메서드 구현 (http.Client 사용)
- [x] 2.5 FetchResult에 ContentType 포함하여 반환

## 3. 테스트 HTML fixture 생성

- [x] 2.1 `apps/api/internal/bot/crawler/testdata/` 디렉토리 생성
- [x] 2.2 기본 사이트 구조 HTML 파일 생성 (index.html, page1.html, page2.html)
- [x] 2.3 depth 2 페이지가 포함된 하위 디렉토리 및 HTML 파일 생성
- [x] 2.4 외부 도메인 링크가 포함된 테스트 페이지 생성
- [x] 2.5 서브도메인 링크가 포함된 테스트 페이지 생성

## 3. 테스트 HTML fixture 생성

- [x] 3.1 `apps/api/internal/bot/crawler/testdata/` 디렉토리 생성
- [x] 3.2 기본 사이트 구조 HTML 파일 생성 (index.html, page1.html, page2.html)
- [x] 3.3 depth 2 페이지가 포함된 하위 디렉토리 및 HTML 파일 생성
- [x] 3.4 외부 도메인 링크가 포함된 테스트 페이지 생성
- [x] 3.5 서브도메인 링크가 포함된 테스트 페이지 생성

## 4. URL 정규화 및 도메인 검증

- [x] 4.1 URL 정규화 함수 구현 (트레일링 슬래시, fragment 제거)
- [x] 4.2 도메인 검증 함수 구현 (hostname 비교)
- [x] 4.3 상대 경로를 절대 URL로 변환하는 함수 구현
- [x] 4.4 파일 확장자 필터링 함수 구현 (.jpg, .png, .pdf 등)

## 5. HTML 링크 추출

- [x] 5.1 HTML 파서를 사용한 a 태그 추출 함수 구현
- [x] 5.2 href 속성에서 URL 추출 및 정규화
- [x] 5.3 빈 href 및 invalid URL 필터링
- [x] 5.4 ContentType이 text/html이 아니면 링크 추출 건너뛰기
- [x] 5.5 링크 추출 단위 테스트 작성

## 6. BFS 탐색 로직

- [x] 6.1 큐(FIFO) 자료구조 초기화
- [x] 6.2 방문한 URL을 추적하는 맵(visited map) 구현
- [x] 6.3 루트 URL을 depth 0으로 큐에 추가하는 로직
- [x] 6.4 큐에서 URL을 꺼내 방문하는 메인 루프 구현
- [x] 6.5 Fetcher.Fetch()로 페이지 조회
- [x] 6.6 최대 depth 제한 검증 로직 추가
- [x] 6.7 방문한 URL에서 링크 추출 및 큐에 추가
- [x] 6.8 중복 URL 체크 및 건너뛰기
- [x] 6.9 각 URL의 부모 URL 및 depth 기록

## 7. 단위 테스트

- [x] 7.1 루트 URL만 방문 (depth 0) 테스트
- [x] 7.2 depth 1까지 방문 테스트
- [x] 7.3 depth 2까지 방문하여 BFS 순서 검증 테스트
- [x] 7.4 최대 depth 제한 준수 테스트
- [x] 7.5 동일 도메인 링크만 따라가는지 테스트
- [x] 7.6 외부 도메인 링크 제외 테스트
- [x] 7.7 서브도메인 링크 제외 테스트
- [x] 7.8 중복 URL 방문 방지 테스트
- [x] 7.9 상대 경로 URL 변환 테스트
- [x] 7.10 트레일링 슬래시 정규화 테스트
- [x] 7.11 파일 확장자 필터링 테스트
- [x] 7.12 Fetch 실패 (에러) 처리 테스트
- [x] 7.13 빈 사이트 (링크 없음) 테스트
- [x] 7.14 ContentType이 HTML이 아닌 경우 테스트

## 8. Pioneer 통합

NOTE: Pioneer integration deferred - requires careful refactoring of existing code.
The crawler package is complete and tested. Pioneer integration should be done
as a separate focused task to avoid breaking existing functionality.

- [ ] 8.1 `pioneer.go`에서 HTTPFetcher 생성
- [ ] 8.2 `pioneer.go`에서 BFSCrawler 생성 및 사용
- [ ] 8.3 Crawler 결과를 기존 페이지 분류 로직에 전달
- [ ] 8.4 기존 Pioneer integration test로 회귀 검증

## 9. 문서화 및 정리

- [x] 9.1 crawler 패키지 godoc 주석 작성
- [x] 9.2 README.md에 Fetcher 인터페이스 사용 예시 추가
- [x] 9.3 테스트 커버리지 확인 (80% 이상 목표)
