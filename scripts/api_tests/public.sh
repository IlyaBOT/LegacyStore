#!/bin/sh

run_public_tests() {
  test_bootstrap
  test_security_headers
  test_categories
  test_app_list
  test_home_feed
  test_server_rankings
  test_catalog_filters
  test_search
  test_app_detail_versions
  test_compatibility
  test_download_metadata
  test_not_found
  test_https_required
  test_invalid_login_and_json
}

test_bootstrap() {
  NAME='Bootstrap'
  api_request GET '/api/v1/bootstrap' 0
  if ! status_is 200; then
    err "$NAME" HttpStatus 200 "$(cat "$STATUS_FILE") $(cat "$BODY_FILE")"
    return
  fi
  if ! jq_ok '.catalog_format_version and .api_version and .server_status == "ok"'; then
    err "$NAME" JsonShape 'api_version, catalog_format_version, server_status=ok' "$(cat "$BODY_FILE")"
    return
  fi
  ok "$NAME"
}

test_security_headers() {
  NAME='SecurityHeaders'
  api_request GET '/api/v1/bootstrap' 1
  if ! status_is 200; then
    err "$NAME" HttpStatus 200 "$(cat "$STATUS_FILE")"
    return
  fi
  if ! header_has 'X-Content-Type-Options' 'nosniff' || ! header_has 'X-Frame-Options' 'DENY'; then
    err "$NAME" Headers 'nosniff and DENY security headers' "$(cat "$HEADERS_FILE")"
    return
  fi
  if ! grep -Eiq '^Strict-Transport-Security:' "$HEADERS_FILE"; then
    err "$NAME" HSTS 'Strict-Transport-Security on secure request' "$(cat "$HEADERS_FILE")"
    return
  fi
  ok "$NAME"
}

test_categories() {
  NAME='Categories'
  api_request GET '/api/v1/categories' 0
  if ! status_is 200 || ! jq_ok '.categories | type == "array" and length > 0 and .[0].slug and .[0].name'; then
    err "$NAME" Response '200 and non-empty categories array' "$(cat "$STATUS_FILE") $(cat "$BODY_FILE")"
    return
  fi
  ok "$NAME"
}

test_app_list() {
  NAME='AppList'
  api_request GET '/api/v1/apps?os=10.9.5&arch=x86_64&limit=12&page=1' 0
  if ! status_is 200 || ! jq_ok '.apps | type == "array" and length > 0 and .[0].slug and .[0].name and .[0].compatibility'; then
    err "$NAME" Response '200 and app cards with compatibility' "$(cat "$STATUS_FILE") $(cat "$BODY_FILE")"
    return
  fi
  ok "$NAME"
}

test_home_feed() {
  NAME='HomeFeed'
  api_request GET '/api/v1/home?os=10.9.5&arch=x86_64' 0
  if ! status_is 200 || ! jq_ok '
    (.popular | type == "array" and length > 0) and
    (.top_downloads | type == "array" and length > 0) and
    (.new_releases | type == "array" and length > 0) and
    (.slides | type == "array" and length == 4) and
    (.top_downloads[0].slug == "pixelmator") and
    (.new_releases[0].slug == "libreoffice") and
    (.slides | any(.metric == "Top This Week" and .app.slug == "vlc")) and
    (.slides | any(.metric == "Top Today" and .app.slug == "transmission"))
  '; then
    err "$NAME" Response 'ranked home lists and four carousel candidates' "$(cat "$STATUS_FILE") $(cat "$BODY_FILE")"
    return
  fi
  ok "$NAME"
}

test_server_rankings() {
  NAME='ServerRankings'
  api_request GET '/api/v1/apps?os=10.9.5&arch=x86_64&limit=3&page=1&sort=downloads' 0
  if ! status_is 200 || ! jq_ok '.apps[0].slug == "pixelmator" and .apps[0].downloads >= 50'; then
    err "$NAME" AllTimeDownloads 'Pixelmator first by all-time downloads' "$(cat "$STATUS_FILE") $(cat "$BODY_FILE")"
    return
  fi

  api_request GET '/api/v1/apps?os=10.9.5&arch=x86_64&limit=3&page=1&sort=downloads-week' 0
  if ! status_is 200 || ! jq_ok '.apps[0].slug == "vlc"'; then
    err "$NAME" WeeklyDownloads 'VLC first by seven-day downloads' "$(cat "$STATUS_FILE") $(cat "$BODY_FILE")"
    return
  fi

  api_request GET '/api/v1/apps?os=10.9.5&arch=x86_64&limit=3&page=1&sort=downloads-day' 0
  if ! status_is 200 || ! jq_ok '.apps[0].slug == "transmission"'; then
    err "$NAME" DailyDownloads 'Transmission first by 24-hour downloads' "$(cat "$STATUS_FILE") $(cat "$BODY_FILE")"
    return
  fi

  api_request GET '/api/v1/apps?os=10.9.5&arch=x86_64&limit=3&page=1&sort=new' 0
  if ! status_is 200 || ! jq_ok '.apps[0].slug == "libreoffice"'; then
    err "$NAME" NewReleases 'LibreOffice first by fixture release date' "$(cat "$STATUS_FILE") $(cat "$BODY_FILE")"
    return
  fi
  ok "$NAME"
}

test_catalog_filters() {
  NAME='CatalogFilters'
  api_request GET '/api/v1/apps?os=10.9.5&arch=x86_64&limit=3&page=1&category=graphics-design' 0
  if ! status_is 200 || ! jq_ok '.page == 1 and .limit == 3 and (.apps | length <= 3) and (.apps | all(.category == "Graphics & Design"))'; then
    err "$NAME" Filtering 'category filter and pagination' "$(cat "$STATUS_FILE") $(cat "$BODY_FILE")"
    return
  fi
  ok "$NAME"
}

test_search() {
  NAME='Search'
  api_request GET '/api/v1/search?q=vlc&os=10.9.5&arch=x86_64' 0
  if ! status_is 200 || ! jq_ok '.results | type == "array" and any(.slug == "vlc")'; then
    err "$NAME" SearchResult 'VLC in search results' "$(cat "$STATUS_FILE") $(cat "$BODY_FILE")"
    return
  fi
  ok "$NAME"
}

test_app_detail_versions() {
  NAME='AppDetailVersions'
  api_request GET '/api/v1/apps/pixelmator?os=10.9.5&arch=x86_64' 0
  if ! status_is 200 || ! jq_ok '.slug == "pixelmator" and .name and .compatibility and .recommended_artifact.id'; then
    err "$NAME" AppDetail 'Pixelmator detail with recommended artifact' "$(cat "$STATUS_FILE") $(cat "$BODY_FILE")"
    return
  fi
  api_request GET '/api/v1/apps/pixelmator/versions?os=10.9.5&arch=x86_64' 0
  if ! status_is 200 || ! jq_ok '.versions | type == "array" and length > 0 and (.[0].artifacts | type == "array")'; then
    err "$NAME" Versions 'versions array with artifacts' "$(cat "$STATUS_FILE") $(cat "$BODY_FILE")"
    return
  fi
  ok "$NAME"
}

test_compatibility() {
  NAME='Compatibility'
  api_request GET '/api/v1/apps/legacy-32bit-test?os=10.15&arch=x86_64' 0
  if ! status_is 200 || ! jq_ok '.compatibility.status == "blocked" and (.compatibility.reasons | any(.code == "bitness_mismatch" or .code == "requires_32bit"))'; then
    err "$NAME" Catalina32Bit 'blocked 32-bit app on macOS 10.15' "$(cat "$STATUS_FILE") $(cat "$BODY_FILE")"
    return
  fi
  api_request GET '/api/v1/apps?os=10.15&arch=x86_64&compatible=1&limit=50&page=1' 0
  if ! status_is 200 || ! jq_ok '.apps | all(.compatibility.status != "blocked") and all(.slug != "legacy-32bit-test")'; then
    err "$NAME" CompatibleOnly 'blocked apps excluded by compatible=1' "$(cat "$BODY_FILE")"
    return
  fi
  ok "$NAME"
}

test_download_metadata() {
  NAME='DownloadMetadata'
  api_request GET '/api/v1/apps/pixelmator?os=10.9.5&arch=x86_64' 0
  ARTIFACT_ID=$(body_value '.recommended_artifact.id // empty')
  if [ -z "$ARTIFACT_ID" ]; then
    err "$NAME" Fixture 'recommended artifact id' "$(cat "$BODY_FILE")"
    return
  fi
  api_request GET "/api/v1/download/$ARTIFACT_ID?os=10.9.5&arch=x86_64" 0
  if ! status_is 200 || ! jq_ok '.sha256 and .size_bytes and (.download_url or .external_page_url or .source_type)'; then
    err "$NAME" Metadata 'download source, SHA-256 and size' "$(cat "$STATUS_FILE") $(cat "$BODY_FILE")"
    return
  fi
  ok "$NAME"
}

test_not_found() {
  NAME='NotFound'
  api_request GET '/api/v1/apps/this-app-does-not-exist' 0
  if ! status_is 404; then
    err "$NAME" UnknownApp 404 "$(cat "$STATUS_FILE")"
    return
  fi
  api_request GET '/api/v1/download/999999' 0
  if ! status_is 404; then
    err "$NAME" UnknownArtifact 404 "$(cat "$STATUS_FILE")"
    return
  fi
  ok "$NAME"
}

test_https_required() {
  NAME='HttpsRequired'
  api_request POST '/api/v1/auth/register' 0 '' '{"email":"plain-http@example.invalid","password":"password123"}'
  if ! status_is 403 || ! jq_ok '.error == "https_required"'; then
    err "$NAME" Register '403 https_required' "$(cat "$STATUS_FILE") $(cat "$BODY_FILE")"
    return
  fi
  api_request GET '/api/v1/me' 0
  if ! status_is 403 || ! jq_ok '.error == "https_required"'; then
    err "$NAME" ProtectedEndpoint '403 https_required' "$(cat "$STATUS_FILE") $(cat "$BODY_FILE")"
    return
  fi
  ok "$NAME"
}

test_invalid_login_and_json() {
  NAME='InvalidLoginAndJson'
  api_request POST '/api/v1/auth/login' 1 '' '{"email":"invalid@example.invalid","password":"wrong-password"}'
  if ! status_is 401 || ! jq_ok '.error == "invalid_credentials"'; then
    err "$NAME" InvalidLogin '401 invalid_credentials' "$(cat "$STATUS_FILE") $(cat "$BODY_FILE")"
    return
  fi
  UNIQUE_EMAIL="strict-json-$(date +%s)-$$@example.invalid"
  api_request POST '/api/v1/auth/register' 1 '' "{\"email\":\"$UNIQUE_EMAIL\",\"password\":\"password123\",\"unexpected\":true}"
  if ! status_is 400 || ! jq_ok '.error == "invalid_json"'; then
    err "$NAME" UnknownJsonField '400 invalid_json' "$(cat "$STATUS_FILE") $(cat "$BODY_FILE")"
    return
  fi
  ok "$NAME"
}
