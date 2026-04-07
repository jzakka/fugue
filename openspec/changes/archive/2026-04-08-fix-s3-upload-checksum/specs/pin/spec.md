## MODIFIED Requirements

### Requirement: Media upload to S3
시스템은 미디어 파일을 S3 호환 스토리지에 업로드할 때, Body로 seekable reader(`io.ReadSeeker`)를 전달하여 SDK의 checksum 계산이 정상 동작하도록 해야 한다(SHALL). MIME 감지를 위해 헤더를 선읽기(pre-read)한 뒤 전체 파일을 `[]byte`로 버퍼링하고 `bytes.Reader`로 변환하여 `PutObject`에 전달한다.

#### Scenario: 로컬 MinIO(HTTP)에서 이미지 업로드 성공
- **WHEN** 사용자가 JPEG 이미지를 첨부하여 핀 생성 API(`POST /api/pins`)를 호출한다
- **THEN** S3 PutObject가 성공하고, 핀이 정상 생성되어 200 응답을 반환한다

#### Scenario: 오디오 파일 업로드 성공
- **WHEN** 사용자가 MP3 오디오 파일을 첨부하여 핀 생성 API를 호출한다
- **THEN** S3 PutObject가 성공하고, 핀이 정상 생성된다

#### Scenario: 허용되지 않는 파일 타입 거부
- **WHEN** 사용자가 허용 목록에 없는 파일 타입(예: text/plain)을 업로드한다
- **THEN** 시스템은 업로드를 거부하고 적절한 에러를 반환한다
