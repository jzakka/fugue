-- scheduler-frontier-table down:
-- FK 제약상 조인 테이블을 먼저 삭제해야 한다.
DROP TABLE IF EXISTS harvester_frontier_pins;
DROP TABLE IF EXISTS harvester_frontier;
DROP TABLE IF EXISTS pioneer_frontier;
