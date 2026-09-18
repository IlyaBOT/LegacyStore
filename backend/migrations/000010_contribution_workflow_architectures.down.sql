BEGIN;

DROP TRIGGER IF EXISTS staged_uploads_set_updated_at ON staged_uploads;
DROP TABLE IF EXISTS staged_uploads;

ALTER TABLE artifacts
    ADD COLUMN arch_i386 boolean NOT NULL DEFAULT false,
    ADD COLUMN arch_x86_64 boolean NOT NULL DEFAULT false,
    ADD COLUMN supports_32bit boolean NOT NULL DEFAULT false,
    ADD COLUMN supports_64bit boolean NOT NULL DEFAULT false;

UPDATE artifacts
SET arch_i386 = architectures && ARRAY['i386', 'i686']::text[],
    arch_x86_64 = architectures && ARRAY['x86_64']::text[],
    supports_32bit = architectures && ARRAY['i386', 'i686', 'ppc', 'ppc-g3', 'ppc-g4', 'ppc-g5']::text[],
    supports_64bit = architectures && ARRAY['x86_64']::text[];

UPDATE artifacts
SET arch_x86_64 = true,
    supports_64bit = true
WHERE NOT arch_i386 AND NOT arch_x86_64;

ALTER TABLE artifacts
    DROP CONSTRAINT IF EXISTS artifacts_architectures_format_check,
    DROP CONSTRAINT IF EXISTS artifacts_architectures_not_empty_check,
    DROP COLUMN architectures,
    ADD CONSTRAINT artifacts_arch_check CHECK (arch_i386 OR arch_x86_64),
    ADD CONSTRAINT artifacts_bitness_check CHECK (supports_32bit OR supports_64bit);

ALTER TABLE apps DROP COLUMN IF EXISTS source_url;

DELETE FROM roles
WHERE name = 'uploader'
  AND NOT EXISTS (SELECT 1 FROM user_roles ur WHERE ur.role_id = roles.id);

ALTER TABLE roles DROP CONSTRAINT IF EXISTS roles_name_check;
ALTER TABLE roles
    ADD CONSTRAINT roles_name_check
    CHECK (name IN ('guest', 'user', 'trusted', 'moder', 'admin'));

COMMIT;
