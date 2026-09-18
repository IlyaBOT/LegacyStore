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
    err "$NAME" AdminLogin 'seed admin session' "$(cat "$STATUS_FILE") $(cat "$BODY_FILE")"
    return
  fi

  api_request GET '/api/v1/admin/dashboard' 1 "$ADMIN_TOKEN"
  if ! status_is 200 || ! jq_ok '.dashboard.users and .dashboard.apps_approved'; then
    err "$NAME" Dashboard 'dashboard counters' "$(cat "$STATUS_FILE") $(cat "$BODY_FILE")"
    return
  fi

  api_request POST "/api/v1/admin/users/$UPLOADER_ID/roles" 1 "$ADMIN_TOKEN" '{"role":"trusted"}'
  if ! status_is 200; then
    err "$NAME" GrantTrusted '200' "$(cat "$STATUS_FILE") $(cat "$BODY_FILE")"
    return
  fi

  APP_SLUG="api-upload-$(date +%s)-$$"
  BUNDLE_ID="com.legacy.apiupload.$(date +%s).$$"
  api_request POST '/api/v1/admin/apps' 1 "$UPLOADER_TOKEN" "{\"slug\":\"$APP_SLUG\",\"name\":\"API Uploaded App\",\"bundle_id\":\"$BUNDLE_ID\",\"developer_name\":\"LegacyStore\",\"summary\":\"API moderation smoke test\",\"description\":\"Temporary app for moderation smoke test.\",\"category_slug\":\"utilities\"}"
  if ! status_is 200 || ! jq_ok '.app.id and .app.moderation_status == "pending"'; then
    err "$NAME" TrustedSubmission 'pending app' "$(cat "$STATUS_FILE") $(cat "$BODY_FILE")"
    return
  fi
  APP_ID=$(body_value '.app.id')

  api_request GET '/api/v1/admin/moderation?status=pending' 1 "$ADMIN_TOKEN"
  QUEUE_ID=$(jq -r --arg id "$APP_ID" '.items[] | select(.entity_type == "app" and .entity_id == $id) | .id' "$BODY_FILE" 2>/dev/null | head -n 1)
  if ! status_is 200 || [ -z "$QUEUE_ID" ]; then
    err "$NAME" Queue 'pending moderation queue item' "$(cat "$STATUS_FILE") $(cat "$BODY_FILE")"
    return
  fi

  api_request POST "/api/v1/admin/moderation/$QUEUE_ID/approve" 1 "$ADMIN_TOKEN" '{"comment":"API approved"}'
  if ! status_is 200 || ! jq_ok '.status == "ok"'; then
    err "$NAME" Approval 'approved status' "$(cat "$STATUS_FILE") $(cat "$BODY_FILE")"
    return
  fi

  api_request POST "/api/v1/admin/apps/$APP_ID/versions" 1 "$UPLOADER_TOKEN" '{"version":"1.0-test","changelog":"Integration upload test"}'
  VERSION_ID=$(body_value '.version.id // empty')
  if ! status_is 201 || [ -z "$VERSION_ID" ]; then
    err "$NAME" CreateVersion '201 and version id' "$(cat "$STATUS_FILE") $(cat "$BODY_FILE")"
    return
  fi

  UPLOAD_FILE="$TMP_DIR/LegacyStore-API-Test.dmg"
  printf 'LegacyStore integration artifact\n' > "$UPLOAD_FILE"
  multipart_test_upload "/api/v1/admin/versions/$VERSION_ID/upload" "$UPLOADER_TOKEN" "$UPLOAD_FILE"
  ARTIFACT_ID=$(body_value '.artifact.id // empty')
  if ! status_is 200 || [ -z "$ARTIFACT_ID" ] || ! jq_ok '.artifact.moderation_status == "pending" and .upload.status == "quarantined" and (.upload.sha256 | length == 64)'; then
    err "$NAME" Upload 'quarantined artifact with SHA-256' "$(cat "$STATUS_FILE") $(cat "$BODY_FILE")"
    return
  fi

  api_request GET "/api/v1/files/$ARTIFACT_ID" 0
  if ! status_is 404; then
    err "$NAME" QuarantineIsolation '404 before artifact approval' "$(cat "$STATUS_FILE") $(cat "$BODY_FILE")"
    return
  fi

  api_request GET '/api/v1/admin/moderation?status=pending' 1 "$ADMIN_TOKEN"
  ARTIFACT_QUEUE_ID=$(jq -r --arg id "$ARTIFACT_ID" '.items[] | select(.entity_type == "artifact" and .entity_id == $id) | .id' "$BODY_FILE" 2>/dev/null | head -n 1)
  if ! status_is 200 || [ -z "$ARTIFACT_QUEUE_ID" ]; then
    err "$NAME" ArtifactQueue 'pending artifact queue item' "$(cat "$STATUS_FILE") $(cat "$BODY_FILE")"
    return
  fi

  api_request POST "/api/v1/admin/moderation/$ARTIFACT_QUEUE_ID/approve" 1 "$ADMIN_TOKEN" '{"comment":"Artifact approved"}'
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
  if ! status_is 200 || ! jq_ok '.downloads == 0 and (.versions[0].downloads == 0)'; then
    err "$NAME" DownloadMetadataNotCounted 'download metadata request does not increment counters' "$(cat "$STATUS_FILE") $(cat "$BODY_FILE")"
    return
  fi

  api_request GET "/api/v1/files/$ARTIFACT_ID" 0
  if ! status_is 200 || ! cmp -s "$UPLOAD_FILE" "$BODY_FILE"; then
    err "$NAME" LocalFileDownload 'approved file bytes' "$(cat "$STATUS_FILE")"
    return
  fi

  # Repeating the same anonymous full download from the same IP stays unique.
  api_request GET "/api/v1/files/$ARTIFACT_ID" 0
  api_request GET "/api/v1/apps/$APP_SLUG?os=10.9.5&arch=x86_64" 0
  if ! status_is 200 || ! jq_ok '.downloads == 1 and (.versions[0].downloads == 1)'; then
    err "$NAME" AnonymousDownloadUnique 'one completed download per IP and app version' "$(cat "$STATUS_FILE") $(cat "$BODY_FILE")"
    return
  fi

  # A bot-like full fetch is served but must not change analytics.
  api_request GET "/api/v1/files/$ARTIFACT_ID" 0 '' '' 'curl/8.0'
  api_request GET "/api/v1/apps/$APP_SLUG?os=10.9.5&arch=x86_64" 0
  if ! status_is 200 || ! jq_ok '.downloads == 1'; then
    err "$NAME" BotDownloadIgnored 'bot-like user agent excluded from download counter' "$(cat "$STATUS_FILE") $(cat "$BODY_FILE")"
    return
  fi

  # Authenticated users are unique by user + artifact rather than by IP.
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
