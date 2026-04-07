-- WARNING: local dev seed data only. Never run in production!
-- Run: make seed (from project root)
-- seed_tags.sql is executed automatically before this file by Makefile

BEGIN;

-- Clean existing data (FK order)
TRUNCATE pin_tags, pins, auth_accounts, creators CASCADE;

-- ============================================================
-- Creators (5 + fuguebot system account)
-- ============================================================
INSERT INTO creators (id, nickname, avatar_url) VALUES
('00000000-0000-0000-0000-00000000f096', 'fuguebot', NULL),
('00000000-0000-0000-0000-000000000001', '하루', NULL),
('00000000-0000-0000-0000-000000000002', 'mochi', NULL),
('00000000-0000-0000-0000-000000000003', '제로', NULL),
('00000000-0000-0000-0000-000000000004', 'codex', NULL),
('00000000-0000-0000-0000-000000000005', '소라', NULL);

-- ============================================================
-- OAuth accounts
-- ============================================================
INSERT INTO auth_accounts (id, creator_id, provider, provider_id, email) VALUES
('10000000-0000-0000-0000-000000000001', '00000000-0000-0000-0000-000000000001', 'google', 'google-uid-haru', 'haru@example.com'),
('10000000-0000-0000-0000-000000000002', '00000000-0000-0000-0000-000000000002', 'twitter', 'twitter-uid-mochi', 'mochi@example.com'),
('10000000-0000-0000-0000-000000000003', '00000000-0000-0000-0000-000000000003', 'discord', 'discord-uid-zero', 'zero@example.com'),
('10000000-0000-0000-0000-000000000004', '00000000-0000-0000-0000-000000000004', 'google', 'google-uid-codex', 'codex@example.com'),
('10000000-0000-0000-0000-000000000005', '00000000-0000-0000-0000-000000000005', 'twitter', 'twitter-uid-sora', 'sora@example.com');

-- ============================================================
-- Pins (12, new schema: media_url + media_type, no field/tags/pin_count)
-- ============================================================

-- Audio pins (하루)
INSERT INTO pins (id, creator_id, media_url, media_type, url, title, description, og_image) VALUES
('20000000-0000-0000-0000-000000000001', '00000000-0000-0000-0000-000000000001',
 'audio/seed-dreamscape.mp3', 'audio',
 'https://soundcloud.com/haru/dreamscape', 'Dreamscape',
 '몽환적인 신스팝 트랙. 새벽 감성.', NULL),
('20000000-0000-0000-0000-000000000002', '00000000-0000-0000-0000-000000000001',
 'audio/seed-neon-rain.mp3', 'audio',
 'https://soundcloud.com/haru/neon-rain', 'Neon Rain',
 '사이버펑크 분위기의 비트.', NULL);

-- Image pins (mochi)
INSERT INTO pins (id, creator_id, media_url, media_type, url, title, description, og_image) VALUES
('20000000-0000-0000-0000-000000000003', '00000000-0000-0000-0000-000000000002',
 'https://images.unsplash.com/photo-1579783902614-a3fb3927b6a5?w=400&h=500&fit=crop', 'image',
 'https://www.pixiv.net/artworks/12345678', '밤의 정원',
 '판타지 배경 일러스트. 달빛 아래 정원.',
 'https://images.unsplash.com/photo-1579783902614-a3fb3927b6a5?w=400&h=500&fit=crop'),
('20000000-0000-0000-0000-000000000004', '00000000-0000-0000-0000-000000000002',
 'https://images.unsplash.com/photo-1618005182384-a83a8bd57fbe?w=400&h=600&fit=crop', 'image',
 'https://www.pixiv.net/artworks/87654321', '캐릭터 디자인 - 루나',
 '오리지널 캐릭터 루나의 풀바디 디자인.',
 'https://images.unsplash.com/photo-1618005182384-a83a8bd57fbe?w=400&h=600&fit=crop'),
('20000000-0000-0000-0000-000000000005', '00000000-0000-0000-0000-000000000002',
 'https://images.unsplash.com/photo-1549490349-8643362247b5?w=400&h=350&fit=crop', 'image',
 'https://www.pixiv.net/artworks/11111111', '앨범 자켓 - Dreamscape',
 '하루의 Dreamscape 앨범 자켓 작업.',
 'https://images.unsplash.com/photo-1549490349-8643362247b5?w=400&h=350&fit=crop');

-- Video pins (제로)
INSERT INTO pins (id, creator_id, media_url, media_type, url, title, description, og_image) VALUES
('20000000-0000-0000-0000-000000000006', '00000000-0000-0000-0000-000000000003',
 'video/seed-dreamscape-mv.mp4', 'video',
 'https://www.youtube.com/watch?v=abc123', 'Dreamscape MV',
 '하루 x mochi 콜라보 뮤직비디오.',
 'https://images.unsplash.com/photo-1514320291840-2e0a9bf2a9ae?w=400&h=225&fit=crop'),
('20000000-0000-0000-0000-000000000007', '00000000-0000-0000-0000-000000000003',
 'video/seed-typo-reel.mp4', 'video',
 'https://www.youtube.com/watch?v=def456', '타이포그래피 모션 릴',
 '키네틱 타이포그래피 쇼릴.',
 'https://images.unsplash.com/photo-1574717024653-61fd2cf4d44d?w=400&h=225&fit=crop');

-- Image pins (codex - game/code screenshots)
INSERT INTO pins (id, creator_id, media_url, media_type, url, title, description, og_image) VALUES
('20000000-0000-0000-0000-000000000008', '00000000-0000-0000-0000-000000000004',
 'https://images.unsplash.com/photo-1550745165-9bc0b252726f?w=400&h=300&fit=crop', 'image',
 'https://github.com/codex/pixel-dungeon', 'Pixel Dungeon',
 '2D 로그라이크 게임. Unity 기반.',
 'https://images.unsplash.com/photo-1550745165-9bc0b252726f?w=400&h=300&fit=crop'),
('20000000-0000-0000-0000-000000000009', '00000000-0000-0000-0000-000000000004',
 'https://images.unsplash.com/photo-1470225620780-dba8ba36b745?w=400&h=400&fit=crop', 'image',
 'https://github.com/codex/sound-vis', 'Sound Visualizer',
 '음악 시각화 웹앱. Three.js 기반.',
 'https://images.unsplash.com/photo-1470225620780-dba8ba36b745?w=400&h=400&fit=crop');

-- Image pins (소라 - writing covers)
INSERT INTO pins (id, creator_id, media_url, media_type, url, title, description, og_image) VALUES
('20000000-0000-0000-0000-000000000010', '00000000-0000-0000-0000-000000000005',
 'https://images.unsplash.com/photo-1516414447565-b14be0adf13e?w=400&h=500&fit=crop', 'image',
 'https://twitter.com/sora_writes/status/999', '잊혀진 계절 - 시나리오',
 '보이스드라마 시나리오. 전 4화 완결.', NULL),
('20000000-0000-0000-0000-000000000011', '00000000-0000-0000-0000-000000000005',
 'https://images.unsplash.com/photo-1495474472287-4d71bcdd2085?w=400&h=500&fit=crop', 'image',
 'https://twitter.com/sora_writes/status/888', '비주얼노벨 - 카페 루미에르',
 '비주얼노벨 메인 시나리오. 일상+미스터리.', NULL),
('20000000-0000-0000-0000-000000000012', '00000000-0000-0000-0000-000000000005',
 'https://images.unsplash.com/photo-1511671782779-c97d3d27a1d4?w=400&h=400&fit=crop', 'image',
 'https://twitter.com/sora_writes/status/777', 'Neon Rain 가사',
 '하루의 Neon Rain 작사 작업.', NULL);

-- ============================================================
-- Pin-Tag associations (via tag slugs for readability)
-- ============================================================
INSERT INTO pin_tags (pin_id, tag_id) SELECT '20000000-0000-0000-0000-000000000001', id FROM tags WHERE slug IN ('synthpop', 'dreamy', 'indie', 'electronic');
INSERT INTO pin_tags (pin_id, tag_id) SELECT '20000000-0000-0000-0000-000000000002', id FROM tags WHERE slug IN ('cyberpunk', 'beatmaking', 'electronic');
INSERT INTO pin_tags (pin_id, tag_id) SELECT '20000000-0000-0000-0000-000000000003', id FROM tags WHERE slug IN ('fantasy', 'background-art', 'illustration', 'concept-art');
INSERT INTO pin_tags (pin_id, tag_id) SELECT '20000000-0000-0000-0000-000000000004', id FROM tags WHERE slug IN ('character-design', 'illustration', 'kawaii');
INSERT INTO pin_tags (pin_id, tag_id) SELECT '20000000-0000-0000-0000-000000000005', id FROM tags WHERE slug IN ('album-art', 'commission', 'illustration');
INSERT INTO pin_tags (pin_id, tag_id) SELECT '20000000-0000-0000-0000-000000000006', id FROM tags WHERE slug IN ('music-video', 'motion-graphics', 'collaboration');
INSERT INTO pin_tags (pin_id, tag_id) SELECT '20000000-0000-0000-0000-000000000007', id FROM tags WHERE slug IN ('typography', 'motion-graphics', 'showreel');
INSERT INTO pin_tags (pin_id, tag_id) SELECT '20000000-0000-0000-0000-000000000008', id FROM tags WHERE slug IN ('game', 'unity', 'pixel-art');
INSERT INTO pin_tags (pin_id, tag_id) SELECT '20000000-0000-0000-0000-000000000009', id FROM tags WHERE slug IN ('web-app', 'threejs', 'interactive');
INSERT INTO pin_tags (pin_id, tag_id) SELECT '20000000-0000-0000-0000-000000000010', id FROM tags WHERE slug IN ('voice-drama', 'scenario', 'fantasy');
INSERT INTO pin_tags (pin_id, tag_id) SELECT '20000000-0000-0000-0000-000000000011', id FROM tags WHERE slug IN ('visual-novel', 'scenario', 'romance');
INSERT INTO pin_tags (pin_id, tag_id) SELECT '20000000-0000-0000-0000-000000000012', id FROM tags WHERE slug IN ('collaboration', 'cyberpunk');

COMMIT;
