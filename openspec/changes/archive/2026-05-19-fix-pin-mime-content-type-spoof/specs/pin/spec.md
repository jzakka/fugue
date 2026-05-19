## ADDED Requirements

### Requirement: MIME 타입 위조 방지는 storage 레이어에서 declared와 sniff의 불일치 거부로 enforce된다

핀 미디어 업로드 시 호출자가 storage 레이어에 전달하는 declared `Content-Type`과 서버가 첫 바이트 시퀀스로 sniff한 detected MIME이 모두 빈 문자열이 아니라면, 두 값이 정규화 후 일치해야 한다(SHALL). 불일치 시 storage 레이어는 업로드를 거부하고 에러 메시지에 `"unsupported file type"` 접두를 포함시켜 반환해야 한다(SHALL). 빈 declared `Content-Type`인 경우에는 비교를 수행하지 않고 sniff된 값으로만 허용 목록을 검증해야 한다(SHALL).

거부 시점은 storage가 외부 저장소(S3 등)에 객체를 쓰기 전이어야 한다(SHALL). 거부 에러는 핸들러의 기존 응답 매핑(`"unsupported file type"` 부분 일치 → 400 + "지원하지 않는 파일 형식입니다")으로 흘러야 한다(SHALL).

본 Requirement는 기존 Requirement `미디어 타입을 자동 감지한다`의 Scenario `MIME 타입 위조 방지`가 production에서 enforce되도록 보장하는 wiring 계약이다. 기존 Scenario의 행위 정의(불일치 시 유효성 검사 오류)는 변경하지 않는다.

#### Scenario: declared와 sniff가 일치하면 통과

- **WHEN** 인증된 유저가 PNG 파일을 `Content-Type: image/png`로 업로드하면
- **THEN** storage는 declared와 sniff가 일치함을 확인하고 업로드를 허용한다

#### Scenario: declared와 sniff가 정규화 후 같으면 통과

- **WHEN** 클라이언트가 JPEG 파일을 alias 표기(`image/jpg`, `image/pjpeg`) 또는 WAV를 `audio/x-wav`·`audio/wave`, MP3를 `audio/mp3`, FLAC를 `audio/x-flac`로 업로드하면
- **THEN** storage는 alias를 canonical 표기로 정규화한 뒤 sniff 결과와 비교하여 업로드를 허용한다

#### Scenario: declared가 비어 있으면 비교 미실행

- **WHEN** multipart 클라이언트가 part-level `Content-Type` 헤더를 표기하지 않고 업로드하면
- **THEN** storage는 declared·sniff 비교를 수행하지 않고 sniff된 MIME만으로 허용 목록을 검증한다

#### Scenario: declared와 sniff가 불일치하면 거부

- **WHEN** 클라이언트가 실제로는 WebM 비디오 바이트를 담은 파일을 `Content-Type: image/png`로 표기하여 업로드하면
- **THEN** storage는 declared(`image/png`)와 sniff(`video/webm`)의 불일치를 감지하여 `"unsupported file type"` 접두를 포함한 에러를 반환하고 외부 저장소에 객체를 쓰지 않으며, 핸들러는 400 + "지원하지 않는 파일 형식입니다"를 응답한다

#### Scenario: declared가 declared 카테고리는 같지만 정확한 MIME이 다르면 거부

- **WHEN** 클라이언트가 PNG 바이트를 `Content-Type: image/jpeg`로 표기하여 업로드하면
- **THEN** storage는 두 값이 모두 image 카테고리임에도 정확한 MIME이 다름을 이유로 거부하고 `"unsupported file type"` 접두를 포함한 에러를 반환한다
