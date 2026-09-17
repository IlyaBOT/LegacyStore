BEGIN;

INSERT INTO categories (slug, name, sort_order) VALUES
    ('books', 'Books', 10),
    ('business', 'Business', 20),
    ('developer-tools', 'Developer Tools', 30),
    ('education', 'Education', 40),
    ('entertainment', 'Entertainment', 50),
    ('finance', 'Finance', 60),
    ('food-drink', 'Food & Drink', 70),
    ('games', 'Games', 80),
    ('graphics-design', 'Graphics & Design', 90),
    ('health-fitness', 'Health & Fitness', 100),
    ('lifestyle', 'Lifestyle', 110),
    ('magazines-newspapers', 'Magazines & Newspapers', 120),
    ('medical', 'Medical', 130),
    ('music', 'Music', 140),
    ('navigation', 'Navigation', 150),
    ('news', 'News', 160),
    ('photo-video', 'Photo & Video', 170),
    ('productivity', 'Productivity', 180),
    ('reference', 'Reference', 190),
    ('safari-extensions', 'Safari Extensions', 200),
    ('shopping', 'Shopping', 210),
    ('social-networking', 'Social Networking', 220),
    ('sports', 'Sports', 230),
    ('travel', 'Travel', 240),
    ('utilities', 'Utilities', 250),
    ('weather', 'Weather', 260)
ON CONFLICT (slug) DO UPDATE
SET name = EXCLUDED.name,
    sort_order = EXCLUDED.sort_order,
    updated_at = now();

INSERT INTO app_categories (app_id, category_id)
SELECT ac.app_id,
       target.id
FROM app_categories ac
JOIN categories legacy ON legacy.id = ac.category_id
JOIN categories target ON target.slug = CASE legacy.slug
    WHEN 'audio-video' THEN 'photo-video'
    WHEN 'photography' THEN 'photo-video'
    WHEN 'internet-network' THEN 'utilities'
END
WHERE legacy.slug IN ('audio-video', 'photography', 'internet-network')
ON CONFLICT (app_id, category_id) DO NOTHING;

DELETE FROM app_categories ac
USING categories legacy
WHERE ac.category_id = legacy.id
  AND legacy.slug IN ('audio-video', 'photography', 'internet-network');

DELETE FROM categories
WHERE slug IN ('audio-video', 'photography', 'internet-network');

COMMIT;
