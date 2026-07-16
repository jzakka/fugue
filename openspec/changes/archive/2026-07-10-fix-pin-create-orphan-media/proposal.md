# fix-pin-create-orphan-media — Proposal

## Why

핀 생성(`POST /api/pins`)에서 미디어 업로드가 일부 입력 검증과 DB 기록보다 먼저 수행되어, 업로드 이후 단계가 실패하면 스토리지에 어떤 핀에도 연결되지 않은 orphan object가 남는다(NAV-1247). orphan은 사용자에게 보이지 않지만 스토리지 비용과 정리 부채로 누적되며, 현재는 발생 사실조차 추적되지 않는다.

## What Changes

- 핀 생성 요청의 모든 폼 필드 검증(설명·URL·og_image 길이 등)을 미디어 업로드보다 먼저 수행하여, 검증 실패(4xx)로 거절되는 요청은 스토리지에 아무 객체도 만들지 않게 한다
- 업로드 이후 단계(DB 기록, 태그 연결)가 실패하면 이미 업로드된 미디어 객체(썸네일 포함)를 보상 삭제한다
- 보상 삭제 자체가 실패하는 경우 운영자가 식별할 수 있는 기록을 남긴다 (응답 결과에는 영향 없음)
- 스토리지 클라이언트에 객체 삭제 기능 추가

## Capabilities

### New Capabilities

(없음)

### Modified Capabilities

- `pin`: 핀 생성의 orphan 미디어 방지 요구사항 추가 — 검증 실패 시 객체 미생성, 업로드 후 실패 시 보상 삭제, 보상 삭제 실패 시 기록

## Impact

- `apps/api/internal/pin/handler.go` — Create 흐름의 검증 순서 조정 및 실패 경로 보상 삭제
- `apps/api/internal/storage/storage.go` — 객체 삭제 기능 추가
- `apps/api/internal/storage/storage_test.go` — 객체 삭제 테스트
- `apps/api/internal/pin/handler_test.go` — 검증 순서·보상 삭제 테스트
- API 계약(요청/응답 스키마) 변경 없음, DB 스키마 변경 없음
