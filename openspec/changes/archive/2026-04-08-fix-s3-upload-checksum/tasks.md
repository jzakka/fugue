## 1. Storage 레이어 수정

- [x] 1.1 `storage.go`에 `"bytes"` import 추가 (`strings`는 `TrimRight`에서 사용 중이므로 유지)
- [x] 1.2 `Upload` 메서드에서 `io.MultiReader` → `io.ReadAll` + `bytes.NewReader`로 교체

## 2. 검증

- [x] 2.1 로컬 MinIO 환경에서 이미지 파일 업로드 테스트 (POST /api/pins) — 201 응답 확인
- [x] 2.2 오디오 파일 업로드 테스트 — 201 응답 확인
- [x] 2.3 허용되지 않는 파일 타입 업로드 시 400 에러 반환 확인
