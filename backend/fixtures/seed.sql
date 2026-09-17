BEGIN;

TRUNCATE TABLE
    download_events,
    legacy_devices,
    legacy_passwords,
    sessions,
    artifact_mirrors,
    screenshots,
    icons,
    review_likes,
    review_replies,
    reviews,
    artifacts,
    app_versions,
    app_categories,
    apps,
    categories,
    user_roles,
    users
RESTART IDENTITY CASCADE;

INSERT INTO users (email, password_hash, nickname, email_verified, status) VALUES
    ('admin@legacystore.local', 'pbkdf2_sha256$210000$bGVnYWN5c3RvcmUtZGV2LQ$sQCo4mcka2r5JWHC4PTYCpJ/u01nYTZnu7LmxxyXl4Y', 'Administrator', true, 'active');

INSERT INTO user_roles (user_id, role_id)
SELECT u.id, r.id
FROM users u
JOIN roles r ON r.name = 'admin'
WHERE u.email = 'admin@legacystore.local';

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
    ('weather', 'Weather', 260);

CREATE TEMP TABLE seed_apps (
    slug text PRIMARY KEY,
    name text NOT NULL,
    bundle_id text NOT NULL,
    developer_name text NOT NULL,
    summary text NOT NULL,
    description text NOT NULL,
    category_slug text NOT NULL,
    website_url text NOT NULL,
    license_type text NOT NULL,
    icon_url text NOT NULL
) ON COMMIT DROP;

INSERT INTO seed_apps VALUES
    ('pixelmator', 'Pixelmator', 'com.pixelmatorteam.pixelmator', 'Pixelmator Team', 'Image editing for classic Intel Macs.', 'A graphics editor with multiple historical versions for older Intel Macs.', 'graphics-design', 'https://example.invalid/pixelmator', 'commercial', '/icons/pixelmator.png'),
    ('vlc', 'VLC', 'org.videolan.vlc', 'VideoLAN', 'Media player for many legacy macOS releases.', 'Open media player with broad codec support and older Mac builds.', 'photo-video', 'https://www.videolan.org/vlc/', 'gpl', '/icons/vlc.png'),
    ('cyberduck', 'Cyberduck', 'ch.sudo.cyberduck', 'iterate GmbH', 'File transfer client for FTP, SFTP and WebDAV.', 'Network file transfer client for classic Mac workflows.', 'utilities', 'https://cyberduck.io/', 'gpl', '/icons/cyberduck.png'),
    ('firefox-legacy', 'Firefox Legacy', 'org.mozilla.firefox', 'Mozilla', 'Legacy browser builds for old macOS systems.', 'Extended support browser builds for older Intel-compatible releases.', 'utilities', 'https://www.mozilla.org/firefox/', 'mpl', '/icons/firefox-legacy.png'),
    ('adium', 'Adium', 'com.adiumX.adiumX', 'Adium Team', 'Classic multi-protocol chat client.', 'Messaging client for social networking and chat accounts.', 'social-networking', 'https://adium.im/', 'gpl', '/icons/adium.png'),
    ('transmission', 'Transmission', 'org.m0k.transmission', 'Transmission Project', 'Lightweight BitTorrent client.', 'Download client metadata for older macOS releases. Real P2P transfer is outside this LegacyStore build.', 'utilities', 'https://transmissionbt.com/', 'gpl', '/icons/transmission.png'),
    ('libreoffice', 'LibreOffice', 'org.libreoffice.script', 'The Document Foundation', 'Office suite for documents and spreadsheets.', 'Productivity suite with modern and older compatible versions.', 'productivity', 'https://www.libreoffice.org/', 'mpl', '/icons/libreoffice.png'),
    ('handbrake', 'HandBrake', 'fr.handbrake.HandBrake', 'HandBrake Team', 'Video transcoder for older Intel Macs.', 'Audio and video conversion utility for legacy systems.', 'photo-video', 'https://handbrake.fr/', 'gpl', '/icons/handbrake.png'),
    ('gimp', 'GIMP', 'org.gimp.gimp-2.10', 'GIMP Team', 'Image manipulation program.', 'Graphics application for editing, retouching and image conversion.', 'graphics-design', 'https://www.gimp.org/', 'gpl', '/icons/gimp.png'),
    ('appcleaner', 'AppCleaner', 'net.freemacsoft.AppCleaner', 'FreeMacSoft', 'Utility for removing application support files.', 'Utilities app with a broad compatibility range.', 'utilities', 'https://freemacsoft.net/appcleaner/', 'freeware', '/icons/appcleaner.png'),
    ('legacy-32bit-test', 'Classic 32-bit Utility', 'org.legacystore.legacy32', 'LegacyStore', 'A 32-bit-only utility for older Intel Macs.', 'Small legacy utility that is intentionally unavailable on macOS Catalina.', 'developer-tools', 'https://example.invalid/legacy-32bit-utility', 'freeware', '/icons/legacy-32bit-test.png');

INSERT INTO apps (
    slug,
    name,
    bundle_id,
    developer_name,
    summary,
    description,
    website_url,
    license_type,
    moderation_status
)
SELECT
    slug,
    name,
    bundle_id,
    developer_name,
    summary,
    description,
    website_url,
    license_type,
    'approved'
FROM seed_apps;

INSERT INTO app_categories (app_id, category_id)
SELECT a.id, c.id
FROM seed_apps sa
JOIN apps a ON a.slug = sa.slug
JOIN categories c ON c.slug = sa.category_slug;

CREATE TEMP TABLE seed_versions (
    app_slug text NOT NULL,
    version text NOT NULL,
    release_date date NOT NULL,
    changelog text NOT NULL,
    is_recommended boolean NOT NULL
) ON COMMIT DROP;

INSERT INTO seed_versions VALUES
    ('pixelmator', '3.6', '2016-05-12', 'Last recommended build for OS X Mavericks-era systems.', true),
    ('pixelmator', '2.2', '2013-01-15', 'Older Mountain Lion-era build.', false),
    ('vlc', '2.2.8', '2017-11-22', 'Legacy VLC branch.', true),
    ('cyberduck', '4.7', '2015-06-15', 'Legacy Cyberduck build.', true),
    ('firefox-legacy', '45.9', '2017-04-19', 'Extended support build.', true),
    ('adium', '1.5.10', '2014-05-19', 'Classic Adium build.', true),
    ('transmission', '2.84', '2014-07-02', 'Legacy Transmission build.', true),
    ('libreoffice', '5.4.7', '2018-05-17', 'Office suite build for older Intel Macs.', true),
    ('handbrake', '0.10.5', '2016-02-11', 'Legacy HandBrake build.', true),
    ('gimp', '2.8.22', '2017-05-11', 'Legacy GIMP build.', true),
    ('appcleaner', '3.4', '2017-09-01', 'Utility build for older systems.', true),
    ('legacy-32bit-test', '1.0', '2010-01-01', 'Intentional 32-bit-only build.', true);

INSERT INTO app_versions (app_id, version, release_date, changelog, is_recommended)
SELECT a.id, sv.version, sv.release_date, sv.changelog, sv.is_recommended
FROM seed_versions sv
JOIN apps a ON a.slug = sv.app_slug;

CREATE TEMP TABLE seed_artifacts (
    app_slug text NOT NULL,
    version text NOT NULL,
    file_name text NOT NULL,
    package_type text NOT NULL,
    source_type text NOT NULL,
    url text NOT NULL,
    size_bytes bigint NOT NULL,
    sha256 char(64) NOT NULL,
    min_os text NOT NULL,
    max_supported_os text,
    max_tested_os text,
    hard_block_above_max boolean NOT NULL,
    arch_i386 boolean NOT NULL,
    arch_x86_64 boolean NOT NULL,
    supports_32bit boolean NOT NULL,
    supports_64bit boolean NOT NULL,
    requires_java boolean NOT NULL,
    install_notes text
) ON COMMIT DROP;

INSERT INTO seed_artifacts VALUES
    ('pixelmator', '3.6', 'Pixelmator_3.6.dmg', 'dmg', 'external_direct', 'https://downloads.example.invalid/pixelmator/Pixelmator_3.6.dmg', 205940326, repeat('a', 64), '10.8', '10.13', '10.13', false, false, true, false, true, false, 'Catalog metadata only; no real file is mirrored.'),
    ('pixelmator', '2.2', 'Pixelmator_2.2.dmg', 'dmg', 'external_direct', 'https://downloads.example.invalid/pixelmator/Pixelmator_2.2.dmg', 172300000, repeat('b', 64), '10.7', '10.9', '10.9', false, false, true, true, true, false, 'Older release metadata.'),
    ('vlc', '2.2.8', 'VLC_2.2.8.dmg', 'dmg', 'external_direct', 'https://downloads.example.invalid/vlc/VLC_2.2.8.dmg', 42100120, repeat('c', 64), '10.6', '10.14', '10.14', false, true, true, true, true, false, 'Catalog metadata only.'),
    ('cyberduck', '4.7', 'Cyberduck_4.7.zip', 'zip', 'external_direct', 'https://downloads.example.invalid/cyberduck/Cyberduck_4.7.zip', 88510000, repeat('d', 64), '10.7', '10.14', '10.14', false, false, true, false, true, false, 'Catalog metadata only.'),
    ('firefox-legacy', '45.9', 'Firefox_45.9.dmg', 'dmg', 'external_direct', 'https://downloads.example.invalid/firefox/Firefox_45.9.dmg', 91500000, repeat('e', 64), '10.6', '10.11', '10.11', false, true, true, true, true, false, 'Catalog metadata only.'),
    ('adium', '1.5.10', 'Adium_1.5.10.dmg', 'dmg', 'external_direct', 'https://downloads.example.invalid/adium/Adium_1.5.10.dmg', 25200000, repeat('f', 64), '10.6', '10.10', '10.10', false, true, true, true, true, false, 'Catalog metadata only.'),
    ('transmission', '2.84', 'Transmission_2.84.dmg', 'dmg', 'external_direct', 'https://downloads.example.invalid/transmission/Transmission_2.84.dmg', 7200000, repeat('1', 64), '10.6', '10.14', '10.14', false, true, true, true, true, false, 'Catalog metadata only.'),
    ('libreoffice', '5.4.7', 'LibreOffice_5.4.7.dmg', 'dmg', 'external_direct', 'https://downloads.example.invalid/libreoffice/LibreOffice_5.4.7.dmg', 221000000, repeat('2', 64), '10.8', '10.13', '10.13', false, false, true, false, true, false, 'Catalog metadata only.'),
    ('handbrake', '0.10.5', 'HandBrake_0.10.5.dmg', 'dmg', 'external_direct', 'https://downloads.example.invalid/handbrake/HandBrake_0.10.5.dmg', 18600000, repeat('3', 64), '10.6', '10.12', '10.12', false, false, true, false, true, false, 'Catalog metadata only.'),
    ('gimp', '2.8.22', 'GIMP_2.8.22.dmg', 'dmg', 'external_direct', 'https://downloads.example.invalid/gimp/GIMP_2.8.22.dmg', 97000000, repeat('4', 64), '10.7', '10.12', '10.12', false, false, true, false, true, false, 'Catalog metadata only.'),
    ('appcleaner', '3.4', 'AppCleaner_3.4.zip', 'zip', 'external_direct', 'https://downloads.example.invalid/appcleaner/AppCleaner_3.4.zip', 4300000, repeat('5', 64), '10.6', '10.14', '10.14', false, true, true, true, true, false, 'Catalog metadata only.'),
    ('legacy-32bit-test', '1.0', 'Legacy32_1.0.zip', 'zip', 'external_direct', 'https://downloads.example.invalid/legacy32/Legacy32_1.0.zip', 1200000, repeat('6', 64), '10.6', '10.14', '10.14', true, true, true, true, false, false, 'Intentionally 32-bit-only release.');

INSERT INTO artifacts (
    app_version_id,
    file_name,
    package_type,
    source_type,
    primary_download_url,
    size_bytes,
    sha256,
    min_os,
    max_supported_os,
    max_tested_os,
    hard_block_above_max,
    arch_i386,
    arch_x86_64,
    supports_32bit,
    supports_64bit,
    requires_java,
    install_notes,
    moderation_status
)
SELECT
    av.id,
    sa.file_name,
    sa.package_type,
    sa.source_type,
    sa.url,
    sa.size_bytes,
    sa.sha256,
    sa.min_os,
    sa.max_supported_os,
    sa.max_tested_os,
    sa.hard_block_above_max,
    sa.arch_i386,
    sa.arch_x86_64,
    sa.supports_32bit,
    sa.supports_64bit,
    sa.requires_java,
    sa.install_notes,
    'approved'
FROM seed_artifacts sa
JOIN apps a ON a.slug = sa.app_slug
JOIN app_versions av ON av.app_id = a.id AND av.version = sa.version;

INSERT INTO artifact_mirrors (artifact_id, mirror_type, url, priority, is_active)
SELECT id, 'official', primary_download_url, 10, true
FROM artifacts
WHERE primary_download_url IS NOT NULL;


-- Stable download history for ranking/feed integration tests:
-- Pixelmator wins all-time, VLC wins the last seven days, Transmission wins the last 24 hours.
INSERT INTO download_events (artifact_id, app_id, created_at)
SELECT ar.id, a.id, now() - interval '45 days' - (g * interval '1 minute')
FROM apps a
JOIN app_versions av ON av.app_id = a.id AND av.is_recommended
JOIN artifacts ar ON ar.app_version_id = av.id
CROSS JOIN generate_series(1, 50) AS g
WHERE a.slug = 'pixelmator';

INSERT INTO download_events (artifact_id, app_id, created_at)
SELECT ar.id, a.id, now() - interval '2 days' - (g * interval '1 minute')
FROM apps a
JOIN app_versions av ON av.app_id = a.id AND av.is_recommended
JOIN artifacts ar ON ar.app_version_id = av.id
CROSS JOIN generate_series(1, 30) AS g
WHERE a.slug = 'vlc';

INSERT INTO download_events (artifact_id, app_id, created_at)
SELECT ar.id, a.id, now() - interval '6 hours' - (g * interval '1 minute')
FROM apps a
JOIN app_versions av ON av.app_id = a.id AND av.is_recommended
JOIN artifacts ar ON ar.app_version_id = av.id
CROSS JOIN generate_series(1, 12) AS g
WHERE a.slug = 'transmission';

INSERT INTO icons (app_id, image_url, width, height)
SELECT a.id, sa.icon_url, 512, 512
FROM seed_apps sa
JOIN apps a ON a.slug = sa.slug;

INSERT INTO screenshots (app_id, image_url, caption, sort_order)
SELECT a.id, '/screenshots/' || a.slug || '-1.png', 'Catalog screenshot', 10
FROM apps a;

COMMIT;
