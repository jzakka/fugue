## ADDED Requirements

### Requirement: 크롤 엔진이 등록된 소스를 순회하며 콘텐츠를 수집한다
크롤 엔진은 활성화된 모든 소스의 시드 URL을 방문하고, 각 소스의 추출 로직으로 콘텐츠를 수집해야 한다(SHALL).

#### Scenario: 정상 크롤 실행
- **WHEN** 크롤 엔진이 실행되면
- **THEN** 활성화된 모든 소스의 시드 URL을 순회하고, 각 페이지에서 콘텐츠를 추출한다

#### Scenario: 비활성 소스 제외
- **WHEN** 소스가 비활성 상태이면
- **THEN** 해당 소스는 크롤 대상에서 제외된다

---

### Requirement: robots.txt를 존중한다
크롤 엔진은 대상 사이트의 robots.txt를 확인하고, 크롤이 허용되지 않은 경로는 방문하지 않아야 한다(SHALL).

#### Scenario: robots.txt에서 차단된 경로
- **WHEN** 대상 사이트의 robots.txt가 특정 경로를 Disallow하면
- **THEN** 크롤 엔진은 해당 경로를 방문하지 않는다

#### Scenario: robots.txt가 없는 사이트
- **WHEN** 대상 사이트에 robots.txt가 없으면
- **THEN** 모든 공개 경로를 크롤할 수 있다

---

### Requirement: rate limit을 준수한다
크롤 엔진은 대상 사이트에 과도한 요청을 보내지 않도록 요청 간 지연을 두어야 한다(SHALL).

#### Scenario: 동일 도메인 요청 간격
- **WHEN** 같은 도메인에 연속 요청을 보낼 때
- **THEN** 소스별로 설정된 최소 간격(기본 1초) 이상 대기한다

---

### Requirement: User-Agent를 명시한다
크롤 엔진은 모든 HTTP 요청에 식별 가능한 User-Agent를 포함해야 한다(SHALL).

#### Scenario: 요청 헤더
- **WHEN** 크롤 엔진이 HTTP 요청을 보내면
- **THEN** User-Agent 헤더에 "Fuguebot/1.0" 이 포함된다

---

### Requirement: 크롤 실행은 API 서버와 별도 프로세스이다
크롤 엔진은 API 서버와 독립된 바이너리로 실행되어야 한다(SHALL). cron 또는 K8s CronJob으로 스케줄링한다.

#### Scenario: 독립 실행
- **WHEN** fuguebot 바이너리를 실행하면
- **THEN** API 서버 없이 독립적으로 크롤을 수행하고 종료한다

---

### Requirement: Source 인터페이스로 플랫폼별 수집 로직을 정의한다
각 플랫폼은 공통 Source 인터페이스를 구현하여 콘텐츠를 수집해야 한다(SHALL). HTML 크롤링, REST API 등 수집 방식은 구현에 위임한다.

#### Scenario: 새 플랫폼 추가
- **WHEN** 새 플랫폼 수집기를 추가하려면
- **THEN** Source 인터페이스를 구현한 struct 하나를 작성하면 된다

---

### Requirement: Source는 콘텐츠 항목을 수집한다
각 Source는 대상 플랫폼에서 미디어 URL, 제목, 설명, 출처 URL을 수집해야 한다(SHALL).

#### Scenario: 콘텐츠 수집 성공
- **WHEN** 크롤 엔진이 Source의 수집 메서드를 호출하면
- **THEN** 미디어 URL, 제목, 설명, 출처 URL이 포함된 항목 목록을 반환한다

#### Scenario: 수집할 콘텐츠가 없는 경우
- **WHEN** 대상 플랫폼에 수집 가능한 콘텐츠가 없으면
- **THEN** 빈 목록을 반환한다

---

### Requirement: MVP에서 2개 플랫폼 플러그인을 제공한다
크롤 엔진에 최소 2개의 서로 다른 구조를 가진 플랫폼 플러그인이 포함되어야 한다(SHALL).

#### Scenario: 다른 구조의 플랫폼
- **WHEN** MVP 플러그인이 2개 구현되면
- **THEN** 각각 다른 수집 방식(예: REST API + HTML 크롤링)을 가진 플랫폼이다

---

### Requirement: 외부 미디어를 다운로드하여 S3에 저장한다
크롤러가 추출한 미디어 URL에서 파일을 다운로드하고 S3 미디어 버킷에 업로드해야 한다(SHALL).

#### Scenario: 이미지 다운로드 성공
- **WHEN** 유효한 이미지 URL을 다운로드하면
- **THEN** 이미지 파일이 S3 미디어 버킷에 저장되고 S3 키가 반환된다

#### Scenario: 오디오 다운로드 성공
- **WHEN** 유효한 오디오 URL을 다운로드하면
- **THEN** 오디오 파일이 S3 미디어 버킷에 저장되고 S3 키가 반환된다

---

### Requirement: 미디어 포맷을 검증한다
다운로드한 파일의 MIME 타입을 확인하여 허용된 포맷만 저장해야 한다(SHALL).

#### Scenario: 허용된 포맷
- **WHEN** 다운로드한 파일이 이미지(JPEG, PNG, WebP, GIF) 또는 오디오(MP3, OGG, WAV) 또는 비디오(MP4, WebM)이면
- **THEN** 저장을 진행한다

#### Scenario: 허용되지 않은 포맷
- **WHEN** 다운로드한 파일이 허용된 포맷이 아니면
- **THEN** 저장을 거부하고 해당 항목을 skip 처리한다

---

### Requirement: 미디어 파일 크기를 제한한다
다운로드한 파일이 설정된 크기 제한을 초과하면 저장을 거부해야 한다(SHALL).

#### Scenario: 크기 초과
- **WHEN** 다운로드한 파일이 크기 제한을 초과하면
- **THEN** 저장을 거부하고 해당 항목을 skip 처리한다

#### Scenario: 크기 제한 내
- **WHEN** 다운로드한 파일이 크기 제한 이내이면
- **THEN** 저장을 진행한다

---

### Requirement: 다운로드 실패 시 해당 항목을 건너뛴다
미디어 다운로드가 실패해도 크롤 전체가 중단되지 않아야 한다(SHALL).

#### Scenario: 네트워크 오류
- **WHEN** 미디어 다운로드 중 네트워크 오류가 발생하면
- **THEN** 해당 항목을 skip하고 에러를 로깅한 뒤 다음 항목을 진행한다

#### Scenario: 404 응답
- **WHEN** 미디어 URL이 404를 반환하면
- **THEN** 해당 항목을 skip하고 다음 항목을 진행한다

---

### Requirement: 크롤 상태를 조회할 수 있다
관리자가 fuguebot의 크롤 상태를 API로 조회할 수 있어야 한다(SHALL). 마지막 크롤 시간, 소스별 수집 건수, 실패율을 포함한다.

#### Scenario: 상태 조회
- **WHEN** 관리자가 크롤 상태 API를 호출하면
- **THEN** 소스별 마지막 크롤 시간, 수집 건수, 실패 건수가 반환된다

#### Scenario: 크롤 이력이 없는 소스
- **WHEN** 아직 한 번도 크롤하지 않은 소스가 있으면
- **THEN** 해당 소스의 상태는 "미실행"으로 표시된다

---

### Requirement: 크롤 소스를 동적으로 관리할 수 있다
관리자가 API로 크롤 소스를 추가, 조회, 삭제할 수 있어야 한다(SHALL).

#### Scenario: 소스 추가
- **WHEN** 관리자가 플랫폼명, 시드 URL, 크롤 주기를 포함하여 소스 추가 API를 호출하면
- **THEN** 새 소스가 등록되고 다음 크롤부터 포함된다

#### Scenario: 소스 목록 조회
- **WHEN** 관리자가 소스 목록 API를 호출하면
- **THEN** 등록된 모든 소스의 이름, 시드 URL, 활성 여부, 마지막 크롤 시간이 반환된다

#### Scenario: 소스 삭제
- **WHEN** 관리자가 소스 삭제 API를 호출하면
- **THEN** 해당 소스가 제거되고 다음 크롤부터 제외된다. 이미 수집된 핀은 유지된다.

---

### Requirement: 크롤 통계를 기록한다
매 크롤 실행마다 통계를 기록해야 한다(SHALL).

#### Scenario: 크롤 완료 후 통계 기록
- **WHEN** 크롤 실행이 완료되면
- **THEN** 소스별 마지막 크롤 시간, 수집 건수, skip 건수, 실패 건수가 기록된다
