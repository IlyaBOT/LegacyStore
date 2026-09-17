BEGIN;

-- Category normalization is intentionally not reversed automatically because
-- collapsing legacy categories into the current Mac App Store taxonomy is not
-- losslessly reversible for existing catalog data.
INSERT INTO categories (slug, name, sort_order) VALUES
    ('audio-video', 'Audio & Video', 10),
    ('internet-network', 'Internet & Network', 80),
    ('photography', 'Photography', 110)
ON CONFLICT (slug) DO NOTHING;

COMMIT;
