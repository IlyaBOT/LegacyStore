BEGIN;

DROP TABLE IF EXISTS review_images;

ALTER TABLE screenshots DROP COLUMN IF EXISTS image_asset_id;
ALTER TABLE icons DROP COLUMN IF EXISTS image_asset_id;
ALTER TABLE users DROP COLUMN IF EXISTS avatar_image_id;

DROP TABLE IF EXISTS image_assets;

COMMIT;
