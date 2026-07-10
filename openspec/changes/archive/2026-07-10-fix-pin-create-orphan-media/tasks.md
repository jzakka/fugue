# fix-pin-create-orphan-media — Tasks

## 1. 스토리지 삭제 기능

- [x] 1.1 `apps/api/internal/storage/storage.go`에 `Delete(ctx, key)` 메서드 추가 (S3 `DeleteObject`)
- [x] 1.2 `apps/api/internal/storage/storage_test.go`에 Delete 동작 테스트 추가 — 기존 관례(nil S3로 사전 검증만 테스트)는 Delete에 부적합하므로, httptest 가짜 S3 엔드포인트(`Config.Endpoint` 주입)로 DeleteObject 요청 발행(버킷·key)을 검증

## 2. 핀 생성 흐름 수정

- [x] 2.1 `apps/api/internal/pin/handler.go`의 `Create`에서 description/url/og_image 길이 검증 블록을 미디어 파일 처리·업로드 앞(제목·태그 검증 직후)으로 이동
- [x] 2.2 `Handler.store`를 `Upload`/`Delete` 최소 인터페이스 타입으로 교체 (`NewHandler` 시그니처 불변 확인)
- [x] 2.3 `CreatePin` 실패 경로에서 업로드된 미디어(+썸네일) 보상 삭제 추가 — `context.WithoutCancel` 사용, 실패 시 key 포함 로그
- [x] 2.4 `LinkPinTag` 실패 롤백 경로에서 핀 row 삭제(`DeletePin`)에 더해 미디어(+썸네일) 보상 삭제 추가 — 두 롤백 모두 `context.WithoutCancel` 사용, 실패 시 key 포함 로그

## 3. 테스트

- [x] 3.1 검증 실패(설명 길이 초과 등) 시 스토리지 업로드가 호출되지 않음을 검증하는 테스트 추가
- [x] 3.2 핀 insert 실패·태그 연결 실패 시 업로드된 객체(미디어·썸네일)가 모두 삭제 호출되는지 검증하는 테스트 추가
- [x] 3.3 보상 삭제 실패 시에도 사용자 응답이 원래 실패 응답으로 유지됨을 검증하는 테스트 추가
- [x] 3.4 `apps/api` 디렉토리에서 `go build ./...` 및 `go test ./...` 통과 확인

## 4. 검증

- [x] 4.1 로컬 환경(docker-compose Postgres + MinIO) 기동이 가능하면 실패 시나리오(설명 501자, 존재하지 않는 태그)로 실제 요청을 보내 스토리지에 orphan 객체가 남지 않는지 QA 확인 (불가 시 사유 명시)
