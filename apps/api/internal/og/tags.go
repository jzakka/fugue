package og

import "strings"

// tagDictionary maps known keywords (lowercase) to display tag names.
// Used for dictionary-based auto-tag extraction from OG title/description.
var tagDictionary = map[string]string{
	// 음악 장르
	"synthpop":      "신스팝",
	"synth pop":     "신스팝",
	"신스팝":         "신스팝",
	"dreampop":      "드림팝",
	"dream pop":     "드림팝",
	"드림팝":         "드림팝",
	"lo-fi":         "lo-fi",
	"lofi":          "lo-fi",
	"로파이":         "lo-fi",
	"ambient":       "앰비언트",
	"앰비언트":       "앰비언트",
	"edm":           "EDM",
	"electronic":    "일렉트로닉",
	"일렉트로닉":     "일렉트로닉",
	"indie":         "인디",
	"인디":           "인디",
	"hip hop":       "힙합",
	"hiphop":        "힙합",
	"힙합":           "힙합",
	"jazz":          "재즈",
	"재즈":           "재즈",
	"classical":     "클래식",
	"클래식":         "클래식",
	"rock":          "록",
	"록":             "록",
	"pop":           "팝",
	"팝":             "팝",
	"r&b":           "R&B",
	"rnb":           "R&B",
	"folk":          "포크",
	"포크":           "포크",
	"acoustic":      "어쿠스틱",
	"어쿠스틱":       "어쿠스틱",
	"orchestra":     "오케스트라",
	"오케스트라":     "오케스트라",
	"piano":         "피아노",
	"피아노":         "피아노",
	"vocal":         "보컬",
	"보컬":           "보컬",
	"instrumental":  "인스트루멘탈",
	"bgm":           "BGM",

	// 미술 스타일
	"pixel art":     "픽셀아트",
	"pixelart":      "픽셀아트",
	"픽셀아트":       "픽셀아트",
	"illustration":  "일러스트",
	"일러스트":       "일러스트",
	"watercolor":    "수채화",
	"수채화":         "수채화",
	"oil painting":  "유화",
	"유화":           "유화",
	"digital art":   "디지털아트",
	"디지털아트":     "디지털아트",
	"concept art":   "컨셉아트",
	"컨셉아트":       "컨셉아트",
	"character design": "캐릭터디자인",
	"캐릭터디자인":   "캐릭터디자인",
	"3d":            "3D",
	"3d art":        "3D",
	"animation":     "애니메이션",
	"애니메이션":     "애니메이션",
	"manga":         "만화",
	"만화":           "만화",
	"comic":         "코믹",
	"sketch":        "스케치",
	"스케치":         "스케치",
	"fanart":        "팬아트",
	"fan art":       "팬아트",
	"팬아트":         "팬아트",
	"portrait":      "인물화",
	"landscape":     "풍경화",

	// 분위기/무드
	"cyberpunk":     "사이버펑크",
	"사이버펑크":     "사이버펑크",
	"retro":         "레트로",
	"레트로":         "레트로",
	"vaporwave":     "베이퍼웨이브",
	"neon":          "네온",
	"네온":           "네온",
	"dark":          "다크",
	"다크":           "다크",
	"minimal":       "미니멀",
	"미니멀":         "미니멀",
	"pastel":        "파스텔",
	"파스텔":         "파스텔",
	"몽환":           "몽환",
	"dreamy":        "몽환",
	"ethereal":      "몽환",
	"gothic":        "고딕",
	"고딕":           "고딕",
	"fantasy":       "판타지",
	"판타지":         "판타지",
	"sci-fi":        "SF",
	"sf":            "SF",
	"horror":        "호러",
	"호러":           "호러",
	"cute":          "큐트",
	"귀여운":         "큐트",
	"chill":         "칠",
	"칠":             "칠",
	"emotional":     "감성",
	"감성":           "감성",
	"새벽":           "새벽감성",

	// 영상
	"mv":            "뮤직비디오",
	"music video":   "뮤직비디오",
	"뮤직비디오":     "뮤직비디오",
	"short film":    "단편영화",
	"단편영화":       "단편영화",
	"vlog":          "브이로그",
	"브이로그":       "브이로그",
	"motion graphic": "모션그래픽",
	"모션그래픽":     "모션그래픽",
	"timelapse":     "타임랩스",

	// 프로그래밍
	"open source":   "오픈소스",
	"오픈소스":       "오픈소스",
	"cli":           "CLI",
	"api":           "API",
	"web app":       "웹앱",
	"웹앱":           "웹앱",
	"game":          "게임",
	"게임":           "게임",
	"shader":        "셰이더",
	"generative":    "제너레이티브",
	"creative coding": "크리에이티브코딩",
	"machine learning": "머신러닝",
	"ai":            "AI",

	// 글
	"novel":         "소설",
	"소설":           "소설",
	"poetry":        "시",
	"시":             "시",
	"essay":         "에세이",
	"에세이":         "에세이",
	"review":        "리뷰",
	"리뷰":           "리뷰",
	"tutorial":      "튜토리얼",
	"튜토리얼":       "튜토리얼",

	// VTuber/서브컬처
	"vtuber":        "VTuber",
	"vocaloid":      "보컬로이드",
	"보컬로이드":     "보컬로이드",
	"utau":          "UTAU",
	"mmd":           "MMD",
	"동인":           "동인",
	"doujin":        "동인",
}

// SuggestTags extracts matching tags from OG title and description
// using the predefined tag dictionary. Returns up to maxTags unique tags.
func SuggestTags(title, description string, maxTags int) []string {
	text := strings.ToLower(title + " " + description)

	seen := make(map[string]bool)
	var result []string

	// Longer keys first for better matching ("synth pop" before "pop")
	// Since Go maps are unordered, we do a simple scan but deduplicate by value.
	for keyword, tag := range tagDictionary {
		if seen[tag] {
			continue
		}
		if strings.Contains(text, strings.ToLower(keyword)) {
			seen[tag] = true
			result = append(result, tag)
			if len(result) >= maxTags {
				break
			}
		}
	}

	return result
}
