## 1. classifyURL 정리
- [x] 1.1 패키지 레벨 정규식 변수 추출
- [x] 1.2 skip 패턴 블록 제거
- [x] 1.3 NodeTypeSkip 상수 보존

## 2. 공유 fetchHTML 추출
- [x] 2.1 공유 함수 추출
- [x] 2.2 Pioneer fetchHTML 래핑
- [x] 2.3 Harvester stub 제거
- [x] 2.4 executeNode 호환성 확인

## 3. crawl() 리팩터링
- [x] 3.1 crawler import 추가
- [x] 3.2 FilterChain 인스턴스 생성
- [x] 3.3 ExtractLinksWithSelectors 교체
- [x] 3.4 인라인 필터 → filterChain.Apply
- [x] 3.5 NodeTypeSkip continue 제거
- [x] 3.6 방문 노드 엣지 생성
- [x] 3.7 복합 우선순위 계산
- [x] 3.8 duplicate key 방어 패턴 보존

## 4. 레거시 코드 정리
- [x] 4.1 parseLinks() 삭제
- [x] 4.2 toNullString() 삭제 (미사용)

## 5. 테스트
- [x] 5.1 TestClassifyURL skip→list 변경
- [x] 5.2 전체 테스트 통과 (166 passed)
- [x] 5.3 빌드 성공
