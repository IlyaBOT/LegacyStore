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

  api_request GET '/api/v1/admin/audit-log' 1 "$ADMIN_TOKEN"
  if ! status_is 200 || ! jq_ok '.items | type == "array" and length > 0'; then
    err "$NAME" AuditLog 'non-empty audit log' "$(cat "$STATUS_FILE") $(cat "$BODY_FILE")"
    return
  fi
  ok "$NAME"
}
