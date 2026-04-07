## Why

Pin 생성 시 S3(MinIO) 업로드가 실패한다. AWS SDK Go v2의 `PutObject`에서 `io.MultiReader`로 만든 non-seekable 스트림에 체크섬을 계산하려다 `"unseekable stream is not supported without TLS and trailing checksum"` 에러가 발생한다. 로컬 MinIO는 HTTP(non-TLS)로 동작하므로 SDK가 trailing checksum 대신 header checksum을 사용하려 하지만, 스트림이 seekable하지 않아 실패한다.

## What Changes

- `storage.Upload()` 메서드에서 `io.MultiReader` 대신 `bytes.Reader`(seekable)로 전체 파일을 버퍼링하여 S3에 업로드
- Content-Type 감지를 위해 헤더를 먼저 읽는 기존 로직은 유지하되, 최종 업로드 Body를 seekable reader로 교체

## Capabilities

### New Capabilities

없음 — 기존 기능의 버그 수정.

### Modified Capabilities

- `pin`: 핀 생성 시 미디어 업로드가 정상 동작하도록 storage 레이어 수정

## Impact

- **코드**: `apps/api/internal/storage/storage.go` — `Upload` 메서드
- **의존성**: 변경 없음 (AWS SDK 버전 유지)
- **API**: 동작 변경 없음, 기존 500 에러가 정상 응답으로 바뀜
- **메모리**: 업로드 파일 전체를 메모리에 버퍼링하므로 대용량 파일 시 메모리 사용량 증가 가능. 단, 기존 `maxFileSize`(20MB) 제한이 있어 실사용 문제 없음
