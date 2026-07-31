package db

import (
	"strings"
	"testing"
)

// 태그 매칭 의미의 검색어 길이 불변성 계약 (openspec/specs/search 예정 스펙,
// change fix-search-tag-match-parity): 핀 검색의 태그 매칭은 similarity 경로
// (3자 이상)와 ILIKE 경로(2자 이하) 모두에서 대소문자 무시 부분 일치여야 하며,
// 두 경로가 동일한 술어를 공유해야 한다. 과거 similarity 경로가 `t.name = $1`
// (대소문자 구분 완전 일치)로 후퇴해 `ar`로는 검색되던 `Art` 태그 핀이 `art`
// 로는 검색되지 않는 회귀가 있었다(NAV-1248).

const tagMatchPredicate = `t.name ILIKE '%' || $1 || '%'`

func TestSearchPinQueries_TagMatchUsesCaseInsensitiveSubstring(t *testing.T) {
	queries := map[string]string{
		"SearchPinsBySimilarity":       searchPinsBySimilarity,
		"SearchPinsWithTagFilter":      searchPinsWithTagFilter,
		"SearchPinsByILIKE":            searchPinsByILIKE,
		"SearchPinsILIKEWithTagFilter": searchPinsILIKEWithTagFilter,
	}

	for name, sqlText := range queries {
		if !strings.Contains(sqlText, tagMatchPredicate) {
			t.Errorf("%s: tag match must use case-insensitive substring predicate %q", name, tagMatchPredicate)
		}
		if strings.Contains(sqlText, "t.name = $1") {
			t.Errorf("%s: tag match must not use case-sensitive exact predicate `t.name = $1`", name)
		}
	}
}

// similarity 경로에서는 결과 포함 판정(WHERE EXISTS)과 태그 일치 가산점(CASE)
// 이 같은 매칭 규칙을 써야 한다. 포함 판정만 넓히고 가산점이 완전 일치로 남으면
// 태그로 걸린 핀이 가산점을 못 받아 순위가 비직관적으로 낮아진다.
func TestSearchPinSimilarityQueries_ScoreAndFilterShareTagPredicate(t *testing.T) {
	for name, sqlText := range map[string]string{
		"SearchPinsBySimilarity":  searchPinsBySimilarity,
		"SearchPinsWithTagFilter": searchPinsWithTagFilter,
	} {
		if got := strings.Count(sqlText, tagMatchPredicate); got != 2 {
			t.Errorf("%s: expected tag predicate in both score CASE and WHERE EXISTS (2 occurrences), got %d", name, got)
		}
	}
}
