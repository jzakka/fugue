CREATE TABLE project_types (
    id              VARCHAR(50) PRIMARY KEY,
    name            VARCHAR(100) NOT NULL,
    required_fields TEXT[] NOT NULL
);

INSERT INTO project_types (id, name, required_fields) VALUES
    ('mv',             'MV',           '{"일러스트","영상"}'),
    ('game',           '게임',          '{"일러스트","음악","사운드","3D"}'),
    ('album-artwork',  '앨범 아트워크',   '{"일러스트"}'),
    ('animation',      '애니메이션',     '{"일러스트","음악","성우"}'),
    ('voice-drama',    '보이스드라마',    '{"성우","음악","사운드"}'),
    ('other',          '기타',          '{}');
