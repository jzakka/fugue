-- Tag seed data for Fugue (creative-focused taxonomy)
-- Categories: 스타일, 장르, 기법, 도구, 분위기, 용도

BEGIN;

TRUNCATE tags CASCADE;

-- ============================================================
-- 스타일 (Style)
-- ============================================================
INSERT INTO tags (name, slug, category, display_order) VALUES
('사이버펑크', 'cyberpunk', '스타일', 1),
('판타지', 'fantasy', '스타일', 2),
('미니멀', 'minimal', '스타일', 3),
('레트로', 'retro', '스타일', 4),
('몽환', 'dreamy', '스타일', 5),
('고딕', 'gothic', '스타일', 6),
('팝아트', 'pop-art', '스타일', 7),
('추상', 'abstract', '스타일', 8),
('사실주의', 'realism', '스타일', 9),
('아르누보', 'art-nouveau', '스타일', 10),
('보텍스', 'vaporwave', '스타일', 11),
('픽셀아트', 'pixel-art', '스타일', 12),
('로파이', 'lofi', '스타일', 13),
('네온', 'neon', '스타일', 14),
('파스텔', 'pastel', '스타일', 15),
('그런지', 'grunge', '스타일', 16),
('카와이', 'kawaii', '스타일', 17),
('다크', 'dark', '스타일', 18),
('빈티지', 'vintage', '스타일', 19),
('퓨처리스틱', 'futuristic', '스타일', 20),
('셀애니', 'cel-anime', '스타일', 21),
('수묵화', 'ink-wash', '스타일', 22),
('플랫디자인', 'flat-design', '스타일', 23),
('이소메트릭', 'isometric', '스타일', 24),
('글리치', 'glitch', '스타일', 25);

-- ============================================================
-- 장르 (Genre)
-- ============================================================
INSERT INTO tags (name, slug, category, display_order) VALUES
('힙합', 'hiphop', '장르', 1),
('일렉트로닉', 'electronic', '장르', 2),
('재즈', 'jazz', '장르', 3),
('클래식', 'classical', '장르', 4),
('앰비언트', 'ambient', '장르', 5),
('록', 'rock', '장르', 6),
('R&B', 'rnb', '장르', 7),
('팝', 'pop', '장르', 8),
('인디', 'indie', '장르', 9),
('EDM', 'edm', '장르', 10),
('신스팝', 'synthpop', '장르', 11),
('메탈', 'metal', '장르', 12),
('포크', 'folk', '장르', 13),
('시티팝', 'citypop', '장르', 14),
('드럼앤베이스', 'drum-and-bass', '장르', 15),
('트랩', 'trap', '장르', 16),
('하우스', 'house', '장르', 17),
('테크노', 'techno', '장르', 18),
('보사노바', 'bossa-nova', '장르', 19),
('OST', 'ost', '장르', 20),
('칠아웃', 'chillout', '장르', 21),
('소울', 'soul', '장르', 22),
('레게', 'reggae', '장르', 23),
('블루스', 'blues', '장르', 24),
('월드뮤직', 'world-music', '장르', 25),
('SF', 'sci-fi', '장르', 26),
('호러', 'horror', '장르', 27),
('로맨스', 'romance', '장르', 28),
('액션', 'action', '장르', 29),
('코미디', 'comedy', '장르', 30);

-- ============================================================
-- 기법 (Technique)
-- ============================================================
INSERT INTO tags (name, slug, category, display_order) VALUES
('수채화', 'watercolor', '기법', 1),
('유화', 'oil-painting', '기법', 2),
('3D모델링', '3d-modeling', '기법', 3),
('모션그래픽', 'motion-graphics', '기법', 4),
('타이포그래피', 'typography', '기법', 5),
('일러스트', 'illustration', '기법', 6),
('벡터', 'vector', '기법', 7),
('콜라주', 'collage', '기법', 8),
('사진', 'photography', '기법', 9),
('드로잉', 'drawing', '기법', 10),
('캘리그래피', 'calligraphy', '기법', 11),
('판화', 'printmaking', '기법', 12),
('스컬프팅', 'sculpting', '기법', 13),
('비트메이킹', 'beatmaking', '기법', 14),
('샘플링', 'sampling', '기법', 15),
('보컬', 'vocal', '기법', 16),
('라이브코딩', 'live-coding', '기법', 17),
('스톱모션', 'stop-motion', '기법', 18),
('로토스코핑', 'rotoscoping', '기법', 19),
('VFX', 'vfx', '기법', 20),
('컨셉아트', 'concept-art', '기법', 21),
('캐릭터디자인', 'character-design', '기법', 22),
('배경아트', 'background-art', '기법', 23),
('UI디자인', 'ui-design', '기법', 24),
('프로토타이핑', 'prototyping', '기법', 25);

-- ============================================================
-- 도구 (Tool)
-- ============================================================
INSERT INTO tags (name, slug, category, display_order) VALUES
('Photoshop', 'photoshop', '도구', 1),
('Blender', 'blender', '도구', 2),
('Unity', 'unity', '도구', 3),
('Ableton', 'ableton', '도구', 4),
('Premiere', 'premiere', '도구', 5),
('After Effects', 'after-effects', '도구', 6),
('Figma', 'figma', '도구', 7),
('Procreate', 'procreate', '도구', 8),
('FL Studio', 'fl-studio', '도구', 9),
('Logic Pro', 'logic-pro', '도구', 10),
('Illustrator', 'illustrator', '도구', 11),
('DaVinci Resolve', 'davinci-resolve', '도구', 12),
('Clip Studio', 'clip-studio', '도구', 13),
('Unreal Engine', 'unreal-engine', '도구', 14),
('Three.js', 'threejs', '도구', 15),
('TouchDesigner', 'touchdesigner', '도구', 16),
('Godot', 'godot', '도구', 17),
('Reason', 'reason', '도구', 18),
('Cinema 4D', 'cinema-4d', '도구', 19),
('Houdini', 'houdini', '도구', 20);

-- ============================================================
-- 분위기 (Mood)
-- ============================================================
INSERT INTO tags (name, slug, category, display_order) VALUES
('따뜻한', 'warm', '분위기', 1),
('어두운', 'dark-mood', '분위기', 2),
('밝은', 'bright', '분위기', 3),
('잔잔한', 'calm', '분위기', 4),
('강렬한', 'intense', '분위기', 5),
('우울한', 'melancholy', '분위기', 6),
('신비로운', 'mysterious', '분위기', 7),
('에너지틱', 'energetic', '분위기', 8),
('평화로운', 'peaceful', '분위기', 9),
('긴장감', 'tension', '분위기', 10),
('감성적', 'emotional', '분위기', 11),
('유쾌한', 'cheerful', '분위기', 12),
('노스탤지어', 'nostalgic', '분위기', 13),
('몰입감', 'immersive', '분위기', 14),
('청량한', 'refreshing', '분위기', 15);

-- ============================================================
-- 용도 (Purpose)
-- ============================================================
INSERT INTO tags (name, slug, category, display_order) VALUES
('앨범아트', 'album-art', '용도', 1),
('게임', 'game', '용도', 2),
('뮤직비디오', 'music-video', '용도', 3),
('포스터', 'poster', '용도', 4),
('배경음', 'bgm', '용도', 5),
('쇼릴', 'showreel', '용도', 6),
('커미션', 'commission', '용도', 7),
('팬아트', 'fanart', '용도', 8),
('프로필', 'profile', '용도', 9),
('실험', 'experimental', '용도', 10),
('튜토리얼', 'tutorial', '용도', 11),
('콜라보', 'collaboration', '용도', 12),
('시나리오', 'scenario', '용도', 13),
('보이스드라마', 'voice-drama', '용도', 14),
('비주얼노벨', 'visual-novel', '용도', 15),
('웹앱', 'web-app', '용도', 16),
('인터랙티브', 'interactive', '용도', 17),
('라이브', 'live', '용도', 18),
('프로토타입', 'prototype', '용도', 19),
('오픈소스', 'open-source', '용도', 20);

COMMIT;
