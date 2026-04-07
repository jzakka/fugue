## Context

`storage.Upload()`는 MIME 감지를 위해 파일의 첫 512바이트를 `io.ReadAtLeast`로 읽은 뒤, `io.MultiReader`로 원본 스트림과 결합하여 S3에 업로드한다. `io.MultiReader`는 `io.Seeker`를 구현하지 않으므로, AWS SDK Go v2가 non-TLS 환경(MinIO HTTP)에서 header checksum을 계산하려 할 때 seek 실패로 에러가 발생한다.

현재 코드 위치: `apps/api/internal/storage/storage.go:141-149`

## Goals / Non-Goals

**Goals:**
- Pin 생성 시 S3 업로드가 정상 동작하도록 수정
- 기존 MIME 감지 로직 유지

**Non-Goals:**
- AWS SDK 버전 업그레이드
- 대용량 파일을 위한 멀티파트 업로드 도입
- TLS 설정 변경

## Decisions

### 전체 파일을 `[]byte`로 버퍼링 후 `bytes.Reader` 사용

**선택**: `io.ReadAll`로 나머지 body를 모두 읽고, 헤더 바이트와 합쳐서 `bytes.NewReader`로 전달

**대안 검토:**
1. ~~`io.MultiReader` 유지 + SDK checksum 비활성화~~ — `s3.PutObjectInput`에 `ChecksumAlgorithm`을 설정해도 non-TLS trailing checksum 문제는 SDK 내부 로직이라 완전 제어 불가. SDK 옵션으로 `RequestChecksumCalculation`를 끌 수 있지만, SDK 버전에 따라 API가 다르고 향후 호환성 불확실.
2. ~~임시 파일로 디스크에 기록~~ — 불필요한 I/O. 최대 100MB(동영상)이지만 이미 `maxBytes` 제한이 있어 메모리 버퍼링으로 충분.
3. **`bytes.Reader` 사용 (선택)** — 가장 단순. `bytes.Reader`는 `io.ReadSeeker`를 구현하므로 SDK가 정상적으로 checksum을 계산할 수 있다. 이미 `maxBytes` 제한(최대 100MB)이 있어 메모리 사용량이 통제된다.

## Risks / Trade-offs

- **메모리 사용 증가** → 기존에도 `io.MultiReader`로 스트리밍하던 것이 전체 버퍼링으로 변경. 단, `maxBytes` 제한(이미지 10MB, 오디오 50MB, 동영상 100MB)이 있어 실질적 위험 낮음.
- **동시 업로드 시 메모리 압박** → 프로덕션 트래픽이 높아지면 고려 필요하나, MVP 단계에서는 문제 없음.
