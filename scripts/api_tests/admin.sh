#!/bin/sh

run_admin_tests() {
  test_admin_moderation
}

test_admin_moderation() {
  NAME='AdminModeration'
  ADMIN_EMAIL=${API_TEST_ADMIN_EMAIL:-admin@legacystore.local}
  ADMIN_PASSWORD=${API_TEST_ADMIN_PASSWORD:-}
  if [ -z "$ADMIN_PASSWORD" ]; then
    normal "$NAME"
    return
  fi

  register_user uploader
  if ! status_is 200 || [ -z "$REGISTER_TOKEN" ]; then
    err "$NAME" RegisterUploader 200 "$(cat "$STATUS_FILE") $(cat "$BODY_FILE")"
    return
  fi
  UPLOADER_TOKEN=$REGISTER_TOKEN
  UPLOADER_ID=$(body_value '.user.id')

  api_request POST '/api/v1/auth/login' 1 '' "{\"email\":\"$ADMIN_EMAIL\",\"password\":\"$ADMIN_PASSWORD\",\"remember_me\":true}"
  ADMIN_TOKEN=$(body_value '.session_token // empty')
  if ! status_is 200 || [ -z "$ADMIN_TOKEN" ] || ! jq_ok '.user.roles | index("admin")'; then
    err "$NAME" AdminLogin 'admin session' "$(cat "$STATUS_FILE") $(cat "$BODY_FILE")"
    return
  fi

  api_request GET '/api/v1/admin/dashboard' 1 "$ADMIN_TOKEN"
  if ! status_is 200 || ! jq_ok '.dashboard.users and .dashboard.apps_approved'; then
    err "$NAME" Dashboard 'dashboard counters' "$(cat "$STATUS_FILE") $(cat "$BODY_FILE")"
    return
  fi

  api_request POST "/api/v1/admin/users/$UPLOADER_ID/roles" 1 "$ADMIN_TOKEN" '{"role":"uploader"}'
  if ! status_is 200 || ! jq_ok '.status == "ok"'; then
    err "$NAME" GrantUploader '200 status=ok' "$(cat "$STATUS_FILE") $(cat "$BODY_FILE")"
    return
  fi

  api_request GET '/api/v1/me' 1 "$UPLOADER_TOKEN"
  if ! status_is 200 || ! jq_ok '.user.roles | index("uploader")'; then
    err "$NAME" UploaderRoleVisible 'uploader role visible to existing session' "$(cat "$STATUS_FILE") $(cat "$BODY_FILE")"
    return
  fi

  APP_SLUG="api-upload-$(date +%s)-$$"
  BUNDLE_ID="com.legacy.apiupload.$(date +%s).$$"
  api_request POST '/api/v1/admin/apps' 1 "$UPLOADER_TOKEN" "{\"slug\":\"$APP_SLUG\",\"name\":\"API Uploaded App\",\"bundle_id\":\"$BUNDLE_ID\",\"developer_name\":\"LegacyStore\",\"summary\":\"API moderation smoke test\",\"description\":\"Temporary app for moderation smoke test.\",\"website_url\":\"https://example.invalid/app\",\"source_url\":\"https://github.com/example/legacy-app\",\"category_slug\":\"utilities\"}"
  if ! status_is 200 || ! jq_ok '.app.id and .app.moderation_status == "pending" and .app.website_url and .app.source_url'; then
    err "$NAME" CreateApplication 'pending app page with shared metadata' "$(cat "$STATUS_FILE") $(cat "$BODY_FILE")"
    return
  fi
  APP_ID=$(body_value '.app.id')

  api_request GET "/api/v1/admin/apps/$APP_ID" 1 "$UPLOADER_TOKEN"
  if ! status_is 200 || ! jq -e --arg id "$APP_ID" '.app.id == ($id|tonumber) and (.versions | type == "array")' "$BODY_FILE" >/dev/null 2>&1; then
    err "$NAME" ContributorApplication 'uploader can open contributor application page' "$(cat "$STATUS_FILE") $(cat "$BODY_FILE")"
    return
  fi

  api_request GET '/api/v1/admin/moderation?status=pending' 1 "$ADMIN_TOKEN"
  QUEUE_ID=$(jq -r --arg id "$APP_ID" '.items[] | select(.entity_type == "app" and .entity_id == $id) | .id' "$BODY_FILE" 2>/dev/null | head -n 1)
  if ! status_is 200 || [ -z "$QUEUE_ID" ]; then
    err "$NAME" AppQueue 'pending application moderation item' "$(cat "$STATUS_FILE") $(cat "$BODY_FILE")"
    return
  fi

  api_request POST "/api/v1/admin/moderation/$QUEUE_ID/approve" 1 "$ADMIN_TOKEN" '{"comment":"Application approved"}'
  if ! status_is 200 || ! jq_ok '.status == "ok"'; then
    err "$NAME" AppApproval 'approved status' "$(cat "$STATUS_FILE") $(cat "$BODY_FILE")"
    return
  fi

  UPLOAD_FILE="$TMP_DIR/LegacyStore-API-Test.dmg"
  printf 'LegacyStore staged contribution artifact\n' > "$UPLOAD_FILE"
  HTTP_STATUS=$(curl -sS -o "$BODY_FILE" -D "$HEADERS_FILE" -w '%{http_code}' \
    -X POST \
    -H 'X-Forwarded-Proto: https' \
    -H "Authorization: Bearer $UPLOADER_TOKEN" \
    -H 'User-Agent: LegacyStore-API-Tests/1.0' \
    -F "file=@$UPLOAD_FILE" \
    "$BASE_URL/api/v1/contributions/apps/$APP_ID/stage")
  printf '%s' "$HTTP_STATUS" > "$STATUS_FILE"
  STAGE_UID=$(body_value '.stage.uid // empty')
  if ! status_is 201 || [ -z "$STAGE_UID" ] || ! jq_ok '.stage.status == "staged" and (.stage.sha256 | length == 64) and .stage.app_id'; then
    err "$NAME" StageRelease '201 staged upload with SHA-256' "$(cat "$STATUS_FILE") $(cat "$BODY_FILE")"
    return
  fi

  api_request GET "/api/v1/contributions/uploads/$STAGE_UID" 1 "$UPLOADER_TOKEN"
  if ! status_is 200 || ! jq_ok --arg uid "$STAGE_UID" '.stage.uid == $uid and .stage.status == "staged"'; then
    err "$NAME" ReadStage 'staged upload belongs to uploader' "$(cat "$STATUS_FILE") $(cat "$BODY_FILE")"
    return
  fi

  api_request POST "/api/v1/contributions/uploads/$STAGE_UID/commit" 1 "$UPLOADER_TOKEN" '{"version":"1.0-test","changelog":"Integration staged upload test","is_recommended":true,"min_os":"10.4","max_supported_os":"10.15","max_tested_os":"10.15","hard_block_above_max":false,"architectures":["i386","x86_64"],"requires_rosetta":false,"requires_java":false,"install_notes":""}'
  VERSION_ID=$(body_value '.release.version_id // empty')
  ARTIFACT_ID=$(body_value '.release.artifact.id // empty')
  if ! status_is 201 || [ -z "$VERSION_ID" ] || [ -z "$ARTIFACT_ID" ] || ! jq_ok '.release.moderation_status == "pending" and (.release.artifact.architectures | index("i386")) and (.release.artifact.architectures | index("x86_64"))'; then
    err "$NAME" CommitRelease 'pending artifact with unified architecture codes' "$(cat "$STATUS_FILE") $(cat "$BODY_FILE")"
    return
  fi

  api_request POST "/api/v1/contributions/uploads/$STAGE_UID/commit" 1 "$UPLOADER_TOKEN" '{"version":"1.0-test","min_os":"10.4","architectures":["x86_64"]}'
  if ! status_is 404; then
    err "$NAME" StageOneShot 'committed stage cannot be committed twice' "$(cat "$STATUS_FILE") $(cat "$BODY_FILE")"
    return
  fi

  api_request GET "/api/v1/files/$ARTIFACT_ID" 0
  if ! status_is 404; then
    err "$NAME" QuarantineIsolation '404 before artifact approval' "$(cat "$STATUS_FILE") $(cat "$BODY_FILE")"
    return
  fi

  if command -v python3 >/dev/null 2>&1; then
    ICON_FILE="$TMP_DIR/test-icon.svg"
    cat > "$ICON_FILE" <<'SVG'
<svg xmlns="http://www.w3.org/2000/svg" width="1024" height="768" viewBox="0 0 1024 768">
  <rect width="1024" height="768" rx="64" fill="#4b8bd8"/>
  <circle cx="512" cy="384" r="180" fill="#ffffff"/>
</svg>
SVG
    image_upload_request "/api/v1/admin/apps/$APP_ID/icons/upload" "$UPLOADER_TOKEN" \
      -F "file=@$ICON_FILE;type=image/svg+xml" \
      -F "app_version_id=$VERSION_ID" \
      -F 'min_os=10.4' \
      -F 'max_os=10.15'
    ICON_URL=$(body_value '.image.url // empty')
    if ! status_is 201 || [ -z "$ICON_URL" ] || ! jq_ok '.image.mime_type == "image/svg+xml" and .image.width <= 512 and .image.height <= 512 and .image.size_bytes <= 1048576'; then
      err "$NAME" SVGIcon 'sanitized version-specific SVG icon <=512x512 and <=1MiB' "$(cat "$STATUS_FILE") $(cat "$BODY_FILE")"
      return
    fi

    api_request GET "$ICON_URL" 0
    if ! status_is 200 || ! grep -Eiq '^Content-Type:[[:space:]]*image/svg\+xml' "$HEADERS_FILE" || ! grep -Eiq '^Content-Security-Policy:' "$HEADERS_FILE"; then
      err "$NAME" SVGServing 'SVG content type and restrictive CSP' "$(cat "$STATUS_FILE") $(cat "$HEADERS_FILE")"
      return
    fi

    SCREEN1="$TMP_DIR/screen-1.png"
    SCREEN2="$TMP_DIR/screen-2.png"
    SCREEN3="$TMP_DIR/screen-3.png"
    SCREEN4="$TMP_DIR/screen-4.png"
    make_test_png "$SCREEN1" 1600 1000
    make_test_png "$SCREEN2" 1440 900
    make_test_png "$SCREEN3" 1280 800
    make_test_png "$SCREEN4" 1024 768

    image_upload_request "/api/v1/admin/apps/$APP_ID/screenshots/upload" "$UPLOADER_TOKEN" \
      -F "images=@$SCREEN1;type=image/png" \
      -F "images=@$SCREEN2;type=image/png" \
      -F "images=@$SCREEN3;type=image/png" \
      -F "app_version_id=$VERSION_ID" \
      -F 'min_os=10.4' \
      -F 'max_os=10.15'
    if ! status_is 201 || ! jq_ok '(.screenshots | length == 3) and (.images | length == 3) and (.images | all(.mime_type == "image/jpeg" and .size_bytes <= 1048576))'; then
      err "$NAME" Screenshots 'three compressed release screenshots' "$(cat "$STATUS_FILE") $(cat "$BODY_FILE")"
      return
    fi

    image_upload_request "/api/v1/admin/apps/$APP_ID/screenshots/upload" "$UPLOADER_TOKEN" \
      -F "images=@$SCREEN4;type=image/png" \
      -F "app_version_id=$VERSION_ID"
    if ! status_is 400 || ! jq_ok '.error == "screenshot_limit"'; then
      err "$NAME" ScreenshotLimit 'fourth screenshot rejected for the same app version' "$(cat "$STATUS_FILE") $(cat "$BODY_FILE")"
      return
    fi
  fi

  api_request GET "/api/v1/apps/$APP_SLUG?os=10.9.5&arch=x86_64" 0
  if ! status_is 200 || ! jq_ok '.recommended_artifact == null or (.recommended_artifact.id == null)'; then
    err "$NAME" PendingReleaseHidden 'pending release is not public before moderation' "$(cat "$STATUS_FILE") $(cat "$BODY_FILE")"
    return
  fi

  api_request GET '/api/v1/admin/moderation?status=pending' 1 "$ADMIN_TOKEN"
  ARTIFACT_QUEUE_ID=$(jq -r --arg id "$ARTIFACT_ID" '.items[] | select(.entity_type == "artifact" and .entity_id == $id) | .id' "$BODY_FILE" 2>/dev/null | head -n 1)
  if ! status_is 200 || [ -z "$ARTIFACT_QUEUE_ID" ]; then
    err "$NAME" ArtifactQueue 'pending artifact moderation item' "$(cat "$STATUS_FILE") $(cat "$BODY_FILE")"
    return
  fi

  api_request POST "/api/v1/admin/moderation/$ARTIFACT_QUEUE_ID/approve" 1 "$ADMIN_TOKEN" '{"comment":"Release artifact approved"}'
  if ! status_is 200 || ! jq_ok '.status == "ok"'; then
    err "$NAME" ArtifactApproval 'approved status' "$(cat "$STATUS_FILE") $(cat "$BODY_FILE")"
    return
  fi

  api_request GET "/api/v1/download/$ARTIFACT_ID?os=10.9.5&arch=x86_64" 0
  if ! status_is 200 || ! jq_ok '.source_type == "local" and (.download_url | contains("/api/v1/files/")) and (.sha256 | length == 64)'; then
    err "$NAME" LocalDownloadMetadata 'local download URL and SHA-256' "$(cat "$STATUS_FILE") $(cat "$BODY_FILE")"
    return
  fi

  api_request GET "/api/v1/apps/$APP_SLUG?os=10.9.5&arch=x86_64" 0
  if ! status_is 200 || ! jq_ok '.downloads == 0 and (.versions[0].downloads == 0) and (.recommended_artifact.architecture_labels | index("Intel 32 Bit (i386 or i686)")) and (.recommended_artifact.architecture_labels | index("Intel 64 Bit (x86_64)"))'; then
    err "$NAME" PublicRelease 'published release exposes human architecture labels and zero downloads' "$(cat "$STATUS_FILE") $(cat "$BODY_FILE")"
    return
  fi

  api_request GET "/api/v1/files/$ARTIFACT_ID" 0
  if ! status_is 200 || ! cmp -s "$UPLOAD_FILE" "$BODY_FILE"; then
    err "$NAME" LocalFileDownload 'approved staged file bytes' "$(cat "$STATUS_FILE")"
    return
  fi

  api_request GET "/api/v1/files/$ARTIFACT_ID" 0
  api_request GET "/api/v1/apps/$APP_SLUG?os=10.9.5&arch=x86_64" 0
  if ! status_is 200 || ! jq_ok '.downloads == 1 and (.versions[0].downloads == 1)'; then
    err "$NAME" AnonymousDownloadUnique 'one completed download per IP and app version' "$(cat "$STATUS_FILE") $(cat "$BODY_FILE")"
    return
  fi

  api_request GET "/api/v1/files/$ARTIFACT_ID" 0 '' '' 'curl/8.0'
  api_request GET "/api/v1/apps/$APP_SLUG?os=10.9.5&arch=x86_64" 0
  if ! status_is 200 || ! jq_ok '.downloads == 1'; then
    err "$NAME" BotDownloadIgnored 'bot-like user agent excluded from download counter' "$(cat "$STATUS_FILE") $(cat "$BODY_FILE")"
    return
  fi

  api_request GET "/api/v1/files/$ARTIFACT_ID" 1 "$UPLOADER_TOKEN"
  api_request GET "/api/v1/files/$ARTIFACT_ID" 1 "$UPLOADER_TOKEN"
  api_request GET "/api/v1/apps/$APP_SLUG?os=10.9.5&arch=x86_64" 0
  if ! status_is 200 || ! jq_ok '.downloads == 2 and (.versions[0].downloads == 2)'; then
    err "$NAME" AuthenticatedDownloadUnique 'one completed download per user and app version plus anonymous IP count' "$(cat "$STATUS_FILE") $(cat "$BODY_FILE")"
    return
  fi

  api_request GET '/api/v1/admin/audit-log' 1 "$ADMIN_TOKEN"
  if ! status_is 200 || ! jq_ok '.items | type == "array" and length > 0'; then
    err "$NAME" AuditLog 'non-empty audit log' "$(cat "$STATUS_FILE") $(cat "$BODY_FILE")"
    return
  fi

  ok "$NAME"
}
