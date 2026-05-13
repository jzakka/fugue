-- Idempotent bot_sites seed for local crawler bootstrap.
-- Safe to re-run: ON CONFLICT (domain) DO NOTHING preserves existing rows.
-- Domains here MUST match the alias table in cmd/bot/main.go sourceRegistry.

INSERT INTO bot_sites (domain, root_url, active)
VALUES
  ('unsplash.com',         'https://unsplash.com/t/wallpapers',           true),
  ('freemusicarchive.org', 'https://freemusicarchive.org/genre/Electronic', true),
  ('pixiv.net',            'https://www.pixiv.net/ranking.php',           true)
ON CONFLICT (domain) DO NOTHING;
