## ADDED Requirements

### Requirement: 통합 검색 API
검색 엔드포인트는 검색어를 받아 핀, 크리에이터, 보드를 한 번에 검색하여 카테고리별로 반환해야 한다. 유사도 기반 랭킹으로 관련성 높은 결과가 상위에 노출된다.

#### Scenario: 전체 검색 (type=all)
- **WHEN** `GET /api/search?q=로파이&type=all` 요청
- **THEN** `{ "pins": [...], "creators": [...], "boards": [...], "top_tags": [...] }` 구조로 세 카테고리 결과를 반환한다

#### Scenario: 카테고리별 검색
- **WHEN** `GET /api/search?q=로파이&type=pins` 요청
- **THEN** `{ "pins": [...], "has_more": boolean, "top_tags": [...] }` 구조로 핀 결과만 반환한다

#### Scenario: 검색어 없이 요청
- **WHEN** `GET /api/search?q=` (빈 검색어) 요청
- **THEN** 400 에러를 반환한다

#### Scenario: 페이지네이션
- **WHEN** `GET /api/search?q=로파이&type=pins&limit=20&offset=20` 요청
- **THEN** offset 이후의 결과를 limit 개수만큼 반환하고 `has_more`로 추가 결과 여부를 표시한다

### Requirement: 핀 검색 범위
핀 검색은 제목(title)과 태그(tags)를 대상으로 한다. 태그가 정확히 일치하는 핀은 랭킹에서 가산점을 받는다.

#### Scenario: 제목 매칭
- **WHEN** 검색어 "로파이"로 검색
- **THEN** title에 "로파이"가 포함된 핀이 유사도순으로 반환된다

#### Scenario: 태그 매칭 부스트
- **WHEN** 검색어 "로파이"로 검색하고, 핀 A는 title에만 매칭, 핀 B는 tags에 정확히 "로파이"가 있음
- **THEN** 핀 B가 핀 A보다 상위에 랭킹된다

### Requirement: 크리에이터 검색 범위
크리에이터 검색은 닉네임(nickname)을 대상으로 한다.

#### Scenario: 닉네임 매칭
- **WHEN** 검색어 "뮤직러버"로 검색
- **THEN** nickname에 "뮤직러버"가 포함된 크리에이터가 유사도순으로 반환된다

### Requirement: 보드 검색 범위와 보안
보드 검색은 이름(name)을 대상으로 하며, 공개 보드만 검색 결과에 포함된다.

#### Scenario: 공개 보드만 검색
- **WHEN** 검색어 "일러스트 모음"으로 검색
- **THEN** `is_public = true`인 보드만 결과에 포함된다

#### Scenario: 비공개 보드 비노출
- **WHEN** 비공개 보드("내 비밀 보드")가 존재하고 검색어로 검색
- **THEN** 해당 보드는 검색 결과에 절대 포함되지 않는다

### Requirement: 한글 2자 이하 검색어 처리
검색어가 2자 이하(유니코드 rune 단위, 공백 제외)일 때는 부분 문자열 매칭으로 fallback한다.

#### Scenario: 2자 검색어
- **WHEN** 검색어 "밴드"(2자)로 검색
- **THEN** ILIKE 기반 부분 매칭으로 "밴드"가 포함된 결과를 반환한다

#### Scenario: 3자 이상 검색어
- **WHEN** 검색어 "로파이"(3자)로 검색
- **THEN** pg_trgm similarity 기반 유사도 검색으로 결과를 반환하고 유사도순으로 정렬한다

### Requirement: 태그 필터
검색 결과를 사전정의 태그로 필터링할 수 있다. 필터는 AND 조건으로 적용된다.

#### Scenario: 단일 태그 필터
- **WHEN** `GET /api/search?q=로파이&tags=음악` 요청
- **THEN** 검색어 매칭 + tags에 "음악"이 포함된 핀만 반환한다

#### Scenario: 복수 태그 필터
- **WHEN** `GET /api/search?q=로파이&tags=음악,일러스트` 요청
- **THEN** 검색어 매칭 + tags에 "음악"과 "일러스트"가 모두 포함된 핀만 반환한다

#### Scenario: 태그 필터 상한
- **WHEN** 6개 이상의 태그로 필터 요청
- **THEN** 400 에러를 반환한다 (최대 5개)

### Requirement: 상위 태그 집계
검색 결과에서 가장 많이 사용된 태그를 집계하여 반환한다.

#### Scenario: top_tags 반환
- **WHEN** 검색 요청 시
- **THEN** 검색 결과 상위 100건의 태그를 집계하여 빈도 높은 순서대로 최대 10개를 `top_tags` 필드로 반환한다

### Requirement: 자동완성
검색바에 타이핑하면 실시간으로 검색 제안을 보여준다.

#### Scenario: 자동완성 드롭다운
- **WHEN** 검색바에 3자 이상 입력 후 300ms 경과
- **THEN** `GET /api/search?q=&type=all&limit=5` 호출하여 핀 최대 3개, 크리에이터 최대 1개, 보드 최대 1개를 섹션별로 드롭다운에 표시한다

#### Scenario: Enter 키로 전체 검색
- **WHEN** 검색바에서 Enter 키를 누름
- **THEN** `/search?q=검색어` 페이지로 이동하여 전체 검색 결과를 표시한다

### Requirement: 검색 결과 페이지
검색 결과를 카테고리별 탭으로 구분하여 표시한다.

#### Scenario: 카테고리 탭
- **WHEN** `/search?q=로파이` 페이지 접속
- **THEN** "전체", "핀", "크리에이터", "보드" 탭이 표시되고, 기본 탭은 "전체"이다

#### Scenario: 태그 필터 칩
- **WHEN** 검색 결과가 표시됨
- **THEN** 결과 상단에 top_tags 기반 태그 칩이 표시되고, 클릭 시 현재 검색어를 유지하면서 해당 태그를 AND 필터로 추가한다

#### Scenario: Deep Link
- **WHEN** `/search?q=로파이&type=pins&tags=음악` URL로 직접 접속
- **THEN** 해당 검색어, 타입, 태그 필터가 적용된 검색 결과가 표시된다

### Requirement: 최근 검색어
최근 검색어를 로컬에 저장하여 빠르게 재검색할 수 있다.

#### Scenario: 검색어 저장
- **WHEN** 검색을 실행
- **THEN** 검색어가 localStorage에 저장된다 (최대 5개, 최신순)

#### Scenario: 최근 검색어 표시
- **WHEN** 검색바에 포커스하고 입력이 비어있음
- **THEN** 최근 검색어 목록이 드롭다운에 표시된다

#### Scenario: 최근 검색어 삭제
- **WHEN** 최근 검색어 항목의 삭제(X) 버튼을 클릭
- **THEN** 해당 검색어가 localStorage에서 제거된다
