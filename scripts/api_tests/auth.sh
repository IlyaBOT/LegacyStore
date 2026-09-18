#!/bin/sh

run_auth_tests() {
  test_register_session_profile
  test_avatar_upload
  test_two_factor
  test_legacy_scopes_and_revocation
  test_reviews
}

test_register_session_profile() {
  NAME='RegisterSessionProfile'
  register_user session
  if ! status_is 200 || [ -z "$REGISTER_TOKEN" ] || ! jq_ok '.user.email and .session_token'; then
    err "$NAME" Register 'user and session token' "$(cat "$STATUS_FILE") $(cat "$BODY_FILE")"
    return
  fi
  EXPECTED_EMAIL=$REGISTER_EMAIL
  TOKEN=$REGISTER_TOKEN

  api_request GET '/api/v1/me' 1 "$TOKEN"
  if ! status_is 200 || [ "$(body_value '.user.email')" != "$EXPECTED_EMAIL" ]; then
    err "$NAME" Me 'registered user profile' "$(cat "$STATUS_FILE") $(cat "$BODY_FILE")"
    return
  fi

  api_request PATCH '/api/v1/me' 1 "$TOKEN" '{"nickname":"API Profile"}'
  if ! status_is 200 || ! jq_ok '.user.nickname == "API Profile"'; then
    err "$NAME" ProfileUpdate 'updated nickname' "$(cat "$STATUS_FILE") $(cat "$BODY_FILE")"
    return
  fi

  api_request GET '/api/v1/me/sessions' 1 "$TOKEN"
  if ! status_is 200 || ! jq_ok '.sessions | type == "array" and length > 0 and all(.auth_kind == "web")'; then
    err "$NAME" Sessions 'web session list' "$(cat "$STATUS_FILE") $(cat "$BODY_FILE")"
    return
  fi
  ok "$NAME"
}

test_avatar_upload() {
  NAME='AvatarUpload'
  if ! command -v python3 >/dev/null 2>&1; then
    normal "$NAME"
    return
  fi
  register_user avatar
  if ! status_is 200 || [ -z "$REGISTER_TOKEN" ]; then
    err "$NAME" Register 200 "$(cat "$STATUS_FILE") $(cat "$BODY_FILE")"
    return
  fi
  TOKEN=$REGISTER_TOKEN

  AVATAR_FILE="$TMP_DIR/avatar-900x700.png"
  make_test_png "$AVATAR_FILE" 900 700 || { normal "$NAME"; return; }
  image_upload_request '/api/v1/me/avatar' "$TOKEN" -F "file=@$AVATAR_FILE;type=image/png"
  AVATAR_URL=$(body_value '.avatar.url // empty')
  if ! status_is 200 || [ -z "$AVATAR_URL" ] || ! jq_ok '.avatar.width <= 512 and .avatar.height <= 512 and .avatar.size_bytes <= 1048576'; then
    err "$NAME" ResizeCompress 'avatar <=512x512 and <=1MiB' "$(cat "$STATUS_FILE") $(cat "$BODY_FILE")"
    return
  fi

  api_request GET '/api/v1/me' 1 "$TOKEN"
  if ! status_is 200 || ! jq_ok '.user.avatar_url | startswith("/api/v1/images/")'; then
    err "$NAME" ProfileURL 'stored local avatar URL' "$(cat "$STATUS_FILE") $(cat "$BODY_FILE")"
    return
  fi

  api_request GET "$AVATAR_URL" 0
  if ! status_is 200 || ! grep -Eiq '^Content-Type:[[:space:]]*image/(jpeg|png)' "$HEADERS_FILE"; then
    err "$NAME" PublicImage 'public processed avatar bytes with image content type' "$(cat "$STATUS_FILE") $(cat "$HEADERS_FILE")"
    return
  fi

  TOO_WIDE="$TMP_DIR/avatar-2049x8.png"
  make_test_png "$TOO_WIDE" 2049 8 || { normal "$NAME"; return; }
  image_upload_request '/api/v1/me/avatar' "$TOKEN" -F "file=@$TOO_WIDE;type=image/png"
  if ! status_is 422 || ! jq_ok '.error == "image_dimensions_too_large"'; then
    err "$NAME" SourceDimensions '422 for source over 2048 pixels on one side' "$(cat "$STATUS_FILE") $(cat "$BODY_FILE")"
    return
  fi

  api_request DELETE '/api/v1/me/avatar' 1 "$TOKEN"
  if ! status_is 200 || ! jq_ok '.status == "deleted"'; then
    err "$NAME" Delete 'avatar deletion' "$(cat "$STATUS_FILE") $(cat "$BODY_FILE")"
    return
  fi
  ok "$NAME"
}

test_two_factor() {
  NAME='TwoFactor'
  if ! command -v python3 >/dev/null 2>&1; then
    normal "$NAME"
    return
  fi
  register_user twofactor
  if ! status_is 200 || [ -z "$REGISTER_TOKEN" ]; then
    err "$NAME" Register 200 "$(cat "$STATUS_FILE") $(cat "$BODY_FILE")"
    return
  fi
  EMAIL=$REGISTER_EMAIL
  PASSWORD=$REGISTER_PASSWORD
  TOKEN=$REGISTER_TOKEN

  api_request POST '/api/v1/auth/2fa/setup' 1 "$TOKEN" "{\"current_password\":\"$PASSWORD\"}"
  if ! status_is 200 || ! jq_ok '.secret and .otpauth_url'; then
    err "$NAME" Setup 'secret and otpauth_url' "$(cat "$STATUS_FILE") $(cat "$BODY_FILE")"
    return
  fi
  SECRET=$(body_value '.secret')
  CODE=$(totp_code "$SECRET")

  api_request POST '/api/v1/auth/2fa/verify' 1 "$TOKEN" "{\"totp_code\":\"$CODE\"}"
  if ! status_is 200 || ! jq_ok '.status == "enabled" and (.recovery_codes | type == "array" and length == 10)'; then
    err "$NAME" Verify '2FA enabled with 10 recovery codes' "$(cat "$STATUS_FILE") $(cat "$BODY_FILE")"
    return
  fi
  RECOVERY_CODE=$(body_value '.recovery_codes[0]')

  api_request POST '/api/v1/auth/login' 1 '' "{\"email\":\"$EMAIL\",\"password\":\"$PASSWORD\"}"
  if ! status_is 401 || ! jq_ok '.error == "two_factor_required"'; then
    err "$NAME" LoginWithoutCode '401 two_factor_required' "$(cat "$STATUS_FILE") $(cat "$BODY_FILE")"
    return
  fi

  CODE=$(totp_code "$SECRET")
  api_request POST '/api/v1/auth/login' 1 '' "{\"email\":\"$EMAIL\",\"password\":\"$PASSWORD\",\"totp_code\":\"$CODE\"}"
  LOGIN_TOKEN=$(body_value '.session_token // empty')
  if ! status_is 200 || [ -z "$LOGIN_TOKEN" ]; then
    err "$NAME" LoginWithCode 'successful login and token' "$(cat "$STATUS_FILE") $(cat "$BODY_FILE")"
    return
  fi

  api_request POST '/api/v1/auth/login' 1 '' "{\"email\":\"$EMAIL\",\"password\":\"$PASSWORD\",\"recovery_code\":\"$RECOVERY_CODE\"}"
  RECOVERY_LOGIN_TOKEN=$(body_value '.session_token // empty')
  if ! status_is 200 || [ -z "$RECOVERY_LOGIN_TOKEN" ]; then
    err "$NAME" LoginWithRecoveryCode 'successful login and token' "$(cat "$STATUS_FILE") $(cat "$BODY_FILE")"
    return
  fi

  api_request POST '/api/v1/auth/login' 1 '' "{\"email\":\"$EMAIL\",\"password\":\"$PASSWORD\",\"recovery_code\":\"$RECOVERY_CODE\"}"
  if ! status_is 401 || ! jq_ok '.error == "two_factor_required"'; then
    err "$NAME" RecoveryCodeSingleUse 'used recovery code rejected' "$(cat "$STATUS_FILE") $(cat "$BODY_FILE")"
    return
  fi

  CODE=$(totp_code "$SECRET")
  api_request POST '/api/v1/auth/2fa/recovery-codes/regenerate' 1 "$TOKEN" "{\"current_password\":\"$PASSWORD\",\"totp_code\":\"$CODE\"}"
  if ! status_is 200 || ! jq_ok '.recovery_codes | type == "array" and length == 10'; then
    err "$NAME" RegenerateRecoveryCodes '10 new recovery codes' "$(cat "$STATUS_FILE") $(cat "$BODY_FILE")"
    return
  fi

  CODE=$(totp_code "$SECRET")
  api_request POST '/api/v1/auth/2fa/disable' 1 "$TOKEN" "{\"current_password\":\"$PASSWORD\",\"totp_code\":\"$CODE\"}"
  if ! status_is 200 || ! jq_ok '.status == "disabled"'; then
    err "$NAME" Disable '2FA disabled' "$(cat "$STATUS_FILE") $(cat "$BODY_FILE")"
    return
  fi
  ok "$NAME"
}

test_legacy_scopes_and_revocation() {
  NAME='LegacyScopesAndRevocation'
  register_user legacy
  if ! status_is 200 || [ -z "$REGISTER_TOKEN" ]; then
    err "$NAME" Register 200 "$(cat "$STATUS_FILE") $(cat "$BODY_FILE")"
    return
  fi
  WEB_TOKEN=$REGISTER_TOKEN
  EMAIL=$REGISTER_EMAIL
  DEVICE_ID="api-device-$$"

  api_request POST '/api/v1/me/legacy-passwords' 1 "$WEB_TOKEN" '{"name":"API Legacy Mac","scopes":["catalog:read","downloads:read","reviews:write","reviews:like","profile:read_basic","admin:any"]}'
  if ! status_is 200 || ! jq_ok '.legacy_password.id and .token and (.legacy_password.scopes | all(. != "admin:any"))'; then
    err "$NAME" CreateLegacyPassword 'one-time token with sanitized scopes' "$(cat "$STATUS_FILE") $(cat "$BODY_FILE")"
    return
  fi
  LEGACY_ID=$(body_value '.legacy_password.id')
  LEGACY_PASSWORD=$(body_value '.token')

  api_request POST '/api/v1/auth/legacy/login' 1 '' "{\"email\":\"$EMAIL\",\"password\":\"$LEGACY_PASSWORD\",\"device_identifier\":\"$DEVICE_ID\",\"device_name\":\"API Mac\"}"
  LEGACY_SESSION=$(body_value '.session_token // empty')
  if ! status_is 200 || [ -z "$LEGACY_SESSION" ]; then
    err "$NAME" LegacyLogin 'legacy session token' "$(cat "$STATUS_FILE") $(cat "$BODY_FILE")"
    return
  fi

  api_request GET '/api/v1/me' 1 "$LEGACY_SESSION"
  if ! status_is 200 || ! jq_ok '.session.auth_kind == "legacy" and (.session.scopes | index("profile:read_basic"))'; then
    err "$NAME" BasicProfile 'legacy basic profile access' "$(cat "$STATUS_FILE") $(cat "$BODY_FILE")"
    return
  fi

  api_request GET '/api/v1/me/devices' 1 "$LEGACY_SESSION"
  if ! status_is 403 || ! jq_ok '.error == "legacy_scope_forbidden"'; then
    err "$NAME" SecurityManagementBlocked 'legacy session cannot manage devices' "$(cat "$STATUS_FILE") $(cat "$BODY_FILE")"
    return
  fi

  api_request GET '/api/v1/admin/dashboard' 1 "$LEGACY_SESSION"
  if ! status_is 403; then
    err "$NAME" AdminBlocked 'legacy session cannot access admin API' "$(cat "$STATUS_FILE") $(cat "$BODY_FILE")"
    return
  fi

  api_request POST '/api/v1/apps/vlc/reviews' 1 "$LEGACY_SESSION" '{"rating":4,"title":"Legacy API","body":"Scoped legacy review."}'
  if ! status_is 200 || ! jq_ok '.review.uid and (.review.uid | length >= 16 and length <= 32)'; then
    err "$NAME" ReviewScope 'reviews:write permits review creation' "$(cat "$STATUS_FILE") $(cat "$BODY_FILE")"
    return
  fi

  api_request GET '/api/v1/me/devices' 1 "$WEB_TOKEN"
  if ! status_is 200 || ! jq_ok '.devices | type == "array" and any(.device_identifier == "'"$DEVICE_ID"'")'; then
    err "$NAME" WebDeviceList 'web session lists legacy device' "$(cat "$STATUS_FILE") $(cat "$BODY_FILE")"
    return
  fi

  api_request POST "/api/v1/me/legacy-passwords/$LEGACY_ID/reset" 1 "$WEB_TOKEN" '{}'
  NEW_LEGACY_PASSWORD=$(body_value '.token // empty')
  if ! status_is 200 || [ -z "$NEW_LEGACY_PASSWORD" ]; then
    err "$NAME" ResetLegacyPassword 'new one-time token' "$(cat "$STATUS_FILE") $(cat "$BODY_FILE")"
    return
  fi

  api_request GET '/api/v1/me' 1 "$LEGACY_SESSION"
  if ! status_is 401; then
    err "$NAME" ResetRevokesSessions 'old legacy session revoked after reset' "$(cat "$STATUS_FILE") $(cat "$BODY_FILE")"
    return
  fi

  api_request POST '/api/v1/auth/legacy/login' 1 '' "{\"email\":\"$EMAIL\",\"password\":\"$NEW_LEGACY_PASSWORD\",\"device_identifier\":\"$DEVICE_ID\",\"device_name\":\"API Mac\"}"
  NEW_LEGACY_SESSION=$(body_value '.session_token // empty')
  if ! status_is 200 || [ -z "$NEW_LEGACY_SESSION" ]; then
    err "$NAME" Relogin 'new legacy password logs in' "$(cat "$STATUS_FILE") $(cat "$BODY_FILE")"
    return
  fi

  api_request DELETE "/api/v1/me/legacy-passwords/$LEGACY_ID" 1 "$WEB_TOKEN"
  if ! status_is 200 || ! jq_ok '.status == "revoked"'; then
    err "$NAME" RevokeLegacyPassword 'revoked status' "$(cat "$STATUS_FILE") $(cat "$BODY_FILE")"
    return
  fi

  api_request GET '/api/v1/me' 1 "$NEW_LEGACY_SESSION"
  if ! status_is 401; then
    err "$NAME" RevokeTerminatesSessions 'legacy sessions revoked with credential' "$(cat "$STATUS_FILE") $(cat "$BODY_FILE")"
    return
  fi
  ok "$NAME"
}

test_reviews() {
  NAME='Reviews'
  register_user reviews
  if ! status_is 200 || [ -z "$REGISTER_TOKEN" ]; then
    err "$NAME" Register 200 "$(cat "$STATUS_FILE") $(cat "$BODY_FILE")"
    return
  fi
  TOKEN=$REGISTER_TOKEN

  api_request POST '/api/v1/apps/pixelmator/reviews' 1 "$TOKEN" '{"rating":5,"title":"API Review","body":"Integration test review.","app_version":"3.6","os_version":"10.9.5","os_arch":"x86_64","device_model":"MacBookPro11,1","client_version":"0.1.7 beta"}'
  REVIEW_UID=$(body_value '.review.uid // empty')
  if ! status_is 200 || [ -z "$REVIEW_UID" ] || [ "${#REVIEW_UID}" -lt 16 ] || [ "${#REVIEW_UID}" -gt 32 ] || ! jq_ok '.review.app_version == "3.6" and .review.os_version == "10.9.5" and .review.os_arch == "x86_64"'; then
    err "$NAME" Create '16-32 char review UID and stored version/system info' "$(cat "$STATUS_FILE") $(cat "$BODY_FILE")"
    return
  fi

  api_request GET '/api/v1/apps/pixelmator/reviews' 0
  if ! status_is 200 || ! jq -e --arg uid "$REVIEW_UID" '.reviews | any(.uid == $uid and .app_version == "3.6" and .author)' "$BODY_FILE" >/dev/null 2>&1; then
    err "$NAME" PublicRead 'public UID, author and app version' "$(cat "$STATUS_FILE") $(cat "$BODY_FILE")"
    return
  fi

  api_request PATCH "/api/v1/reviews/$REVIEW_UID" 1 "$TOKEN" '{"rating":4,"title":"Updated","body":"Updated integration review.","app_version":"3.6","os_version":"10.9.5","os_arch":"x86_64"}'
  if ! status_is 200 || ! jq_ok '.review.rating == 4 and .review.uid'; then
    err "$NAME" Update 'rating=4 with same public UID' "$(cat "$STATUS_FILE") $(cat "$BODY_FILE")"
    return
  fi

  LONG_BODY=$(awk 'BEGIN { for (i = 0; i < 301; i++) printf "x" }')
  api_request PATCH "/api/v1/reviews/$REVIEW_UID" 1 "$TOKEN" "{"rating":4,"body":"$LONG_BODY","app_version":"3.6"}"
  if ! status_is 400; then
    err "$NAME" BodyLimit '400 for review body longer than 300 characters' "$(cat "$STATUS_FILE") $(cat "$BODY_FILE")"
    return
  fi

  if command -v python3 >/dev/null 2>&1; then
    REVIEW_IMG1="$TMP_DIR/review-1.png"
    REVIEW_IMG2="$TMP_DIR/review-2.png"
    REVIEW_IMG3="$TMP_DIR/review-3.png"
    REVIEW_IMG4="$TMP_DIR/review-4.png"
    make_test_png "$REVIEW_IMG1" 1200 800
    make_test_png "$REVIEW_IMG2" 1024 768
    make_test_png "$REVIEW_IMG3" 640 480
    make_test_png "$REVIEW_IMG4" 400 300

    image_upload_request "/api/v1/reviews/$REVIEW_UID/images" "$TOKEN"       -F "images=@$REVIEW_IMG1;type=image/png"       -F "images=@$REVIEW_IMG2;type=image/png"       -F "images=@$REVIEW_IMG3;type=image/png"
    REVIEW_IMAGE_URL=$(body_value '.images[0].url // empty')
    if ! status_is 201 || [ -z "$REVIEW_IMAGE_URL" ] || ! jq_ok '.images | length == 3 and all(.size_bytes <= 1048576 and .width <= 2048 and .height <= 2048)'; then
      err "$NAME" Images 'three compressed review images' "$(cat "$STATUS_FILE") $(cat "$BODY_FILE")"
      return
    fi

    image_upload_request "/api/v1/reviews/$REVIEW_UID/images" "$TOKEN" -F "images=@$REVIEW_IMG4;type=image/png"
    if ! status_is 400 || ! jq_ok '.error == "review_image_limit"'; then
      err "$NAME" ImageLimit 'fourth image rejected' "$(cat "$STATUS_FILE") $(cat "$BODY_FILE")"
      return
    fi

    api_request GET '/api/v1/apps/pixelmator/reviews' 0
    if ! status_is 200 || ! jq -e --arg uid "$REVIEW_UID" '.reviews | any(.uid == $uid and (.images | length == 3))' "$BODY_FILE" >/dev/null 2>&1; then
      err "$NAME" ImageReadback 'review exposes three image URLs' "$(cat "$STATUS_FILE") $(cat "$BODY_FILE")"
      return
    fi

    api_request GET "$REVIEW_IMAGE_URL" 0
    if ! status_is 200 || ! grep -Eiq '^Content-Type:[[:space:]]*image/jpeg' "$HEADERS_FILE"; then
      err "$NAME" ImageServing 'review image served as recompressed JPEG' "$(cat "$STATUS_FILE") $(cat "$HEADERS_FILE")"
      return
    fi
  fi

  api_request POST "/api/v1/reviews/$REVIEW_UID/like" 1 "$TOKEN" '{}'
  if ! status_is 200 || ! jq_ok '.status == "liked"'; then
    err "$NAME" Like 'liked status' "$(cat "$STATUS_FILE") $(cat "$BODY_FILE")"
    return
  fi

  api_request POST "/api/v1/reviews/$REVIEW_UID/replies" 1 "$TOKEN" '{"body":"API reply"}'
  if ! status_is 200 || ! jq_ok '.reply.id'; then
    err "$NAME" Reply 'reply id' "$(cat "$STATUS_FILE") $(cat "$BODY_FILE")"
    return
  fi

  api_request DELETE "/api/v1/reviews/$REVIEW_UID" 1 "$TOKEN"
  if ! status_is 200 || ! jq_ok '.status == "deleted"'; then
    err "$NAME" Delete 'deleted status' "$(cat "$STATUS_FILE") $(cat "$BODY_FILE")"
    return
  fi
  ok "$NAME"
}
