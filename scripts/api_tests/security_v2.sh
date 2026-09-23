#!/bin/sh

run_security_v2_tests() {
  test_account_password_email_recovery
  test_sensitive_endpoints_require_https
  test_signed_catalog
  test_provisioned_admin
}

test_account_password_email_recovery() {
  NAME='AccountSecurityLifecycle'
  register_user accountsec
  if ! status_is 200 || [ -z "$REGISTER_TOKEN" ]; then
    err "$NAME" Register '200 and session token' "$(cat "$STATUS_FILE") $(cat "$BODY_FILE")"
    return
  fi

  OLD_EMAIL=$REGISTER_EMAIL
  OLD_PASSWORD=$REGISTER_PASSWORD
  TOKEN=$REGISTER_TOKEN
  NEW_PASSWORD='replacement horse battery staple'
  RESET_PASSWORD='recovered horse battery staple'
  NEW_EMAIL="changed-$(date +%s)-$$@example.invalid"

  api_request POST '/api/v1/me/password' 1 "$TOKEN" "{\"current_password\":\"wrong password\",\"new_password\":\"$NEW_PASSWORD\"}"
  if ! status_is 401 || ! jq_ok '.error == "invalid_credentials"'; then
    err "$NAME" RejectWrongCurrentPassword '401 invalid_credentials' "$(cat "$STATUS_FILE") $(cat "$BODY_FILE")"
    return
  fi

  api_request POST '/api/v1/me/password' 1 "$TOKEN" "{\"current_password\":\"$OLD_PASSWORD\",\"new_password\":\"$NEW_PASSWORD\"}"
  if ! status_is 200 || ! jq_ok '.status == "password_changed"'; then
    err "$NAME" ChangePassword 'password_changed' "$(cat "$STATUS_FILE") $(cat "$BODY_FILE")"
    return
  fi

  api_request POST '/api/v1/auth/login' 1 '' "{\"email\":\"$OLD_EMAIL\",\"password\":\"$OLD_PASSWORD\"}"
  if ! status_is 401; then
    err "$NAME" OldPasswordRejected '401' "$(cat "$STATUS_FILE") $(cat "$BODY_FILE")"
    return
  fi

  api_request POST '/api/v1/auth/login' 1 '' "{\"email\":\"$OLD_EMAIL\",\"password\":\"$NEW_PASSWORD\"}"
  TOKEN=$(body_value '.session_token // empty')
  if ! status_is 200 || [ -z "$TOKEN" ]; then
    err "$NAME" NewPasswordLogin '200 and session token' "$(cat "$STATUS_FILE") $(cat "$BODY_FILE")"
    return
  fi

  api_request POST '/api/v1/me/email' 1 "$TOKEN" "{\"current_password\":\"$NEW_PASSWORD\",\"new_email\":\"$NEW_EMAIL\"}"
  if ! status_is 200 || ! jq_ok '.status == "email_changed" and .user.email == "'"$NEW_EMAIL"'" and .user.email_verified == false'; then
    err "$NAME" ChangeEmail 'email_changed and email_verified=false' "$(cat "$STATUS_FILE") $(cat "$BODY_FILE")"
    return
  fi

  api_request POST '/api/v1/auth/login' 1 '' "{\"email\":\"$OLD_EMAIL\",\"password\":\"$NEW_PASSWORD\"}"
  if ! status_is 401; then
    err "$NAME" OldEmailRejected '401' "$(cat "$STATUS_FILE") $(cat "$BODY_FILE")"
    return
  fi

  api_request POST '/api/v1/auth/login' 1 '' "{\"email\":\"$NEW_EMAIL\",\"password\":\"$NEW_PASSWORD\"}"
  TOKEN=$(body_value '.session_token // empty')
  if ! status_is 200 || [ -z "$TOKEN" ]; then
    err "$NAME" NewEmailLogin '200 and session token' "$(cat "$STATUS_FILE") $(cat "$BODY_FILE")"
    return
  fi

  api_request POST '/api/v1/auth/recovery/request' 1 '' "{\"email\":\"$NEW_EMAIL\"}"
  RECOVERY_TOKEN=$(body_value '.recovery_token // empty')
  if ! status_is 202 || ! jq_ok '.status == "accepted"' || [ -z "$RECOVERY_TOKEN" ]; then
    err "$NAME" RecoveryRequest '202 accepted and debug recovery token' "$(cat "$STATUS_FILE") $(cat "$BODY_FILE")"
    return
  fi

  api_request POST '/api/v1/auth/recovery/reset' 1 '' "{\"token\":\"$RECOVERY_TOKEN\",\"new_password\":\"$RESET_PASSWORD\"}"
  if ! status_is 200 || ! jq_ok '.status == "password_reset"'; then
    err "$NAME" RecoveryReset 'password_reset' "$(cat "$STATUS_FILE") $(cat "$BODY_FILE")"
    return
  fi

  api_request GET '/api/v1/me' 1 "$TOKEN"
  if ! status_is 401; then
    err "$NAME" RecoveryRevokesSessions 'old session rejected with 401' "$(cat "$STATUS_FILE") $(cat "$BODY_FILE")"
    return
  fi

  api_request POST '/api/v1/auth/recovery/reset' 1 '' "{\"token\":\"$RECOVERY_TOKEN\",\"new_password\":\"another valid password\"}"
  if ! status_is 401 || ! jq_ok '.error == "invalid_recovery_token"'; then
    err "$NAME" RecoveryTokenSingleUse '401 invalid_recovery_token' "$(cat "$STATUS_FILE") $(cat "$BODY_FILE")"
    return
  fi

  api_request POST '/api/v1/auth/login' 1 '' "{\"email\":\"$NEW_EMAIL\",\"password\":\"$RESET_PASSWORD\"}"
  if ! status_is 200 || ! jq_ok '.session_token and .user.email == "'"$NEW_EMAIL"'"'; then
    err "$NAME" RecoveredPasswordLogin 'successful login with recovered password' "$(cat "$STATUS_FILE") $(cat "$BODY_FILE")"
    return
  fi

  api_request POST '/api/v1/auth/recovery/request' 1 '' '{"email":"does-not-exist@example.invalid"}'
  if ! status_is 202 || ! jq_ok '.status == "accepted" and (has("recovery_token") | not)'; then
    err "$NAME" RecoveryEnumerationResistance 'same accepted status without an account-existence signal' "$(cat "$STATUS_FILE") $(cat "$BODY_FILE")"
    return
  fi

  ok "$NAME"
}

test_sensitive_endpoints_require_https() {
  NAME='SensitiveEndpointsRequireHTTPS'
  register_user httpsguard
  if ! status_is 200 || [ -z "$REGISTER_TOKEN" ]; then
    err "$NAME" Register '200 and session token' "$(cat "$STATUS_FILE") $(cat "$BODY_FILE")"
    return
  fi
  TOKEN=$REGISTER_TOKEN
  PASSWORD=$REGISTER_PASSWORD

  api_request POST '/api/v1/me/password' 0 "$TOKEN" "{\"current_password\":\"$PASSWORD\",\"new_password\":\"new secure password\"}"
  if ! status_is 403 || ! jq_ok '.error == "https_required"'; then
    err "$NAME" PasswordChange '403 https_required' "$(cat "$STATUS_FILE") $(cat "$BODY_FILE")"
    return
  fi

  api_request POST '/api/v1/auth/recovery/request' 0 '' '{"email":"nobody@example.invalid"}'
  if ! status_is 403 || ! jq_ok '.error == "https_required"'; then
    err "$NAME" RecoveryRequest '403 https_required' "$(cat "$STATUS_FILE") $(cat "$BODY_FILE")"
    return
  fi

  api_request POST '/api/v1/auth/2fa/setup' 0 "$TOKEN" "{\"current_password\":\"$PASSWORD\"}"
  if ! status_is 403 || ! jq_ok '.error == "https_required"'; then
    err "$NAME" TwoFactorSetup '403 https_required' "$(cat "$STATUS_FILE") $(cat "$BODY_FILE")"
    return
  fi

  ok "$NAME"
}

test_signed_catalog() {
  NAME='SignedCatalog'
  if ! command -v openssl >/dev/null 2>&1 || ! command -v base64 >/dev/null 2>&1; then
    normal "$NAME"
    return
  fi

  api_request GET '/api/v1/bootstrap' 0
  if ! status_is 200 || ! jq_ok '.features.signed_catalog == true'; then
    err "$NAME" Bootstrap 'signed_catalog=true' "$(cat "$STATUS_FILE") $(cat "$BODY_FILE")"
    return
  fi

  api_request GET '/api/v1/catalog/public-key' 0
  if ! status_is 200 || ! jq_ok '.algorithm == "rsa-sha256-pkcs1v15" and .key_id and .pem'; then
    err "$NAME" PublicKey 'RSA signing public key metadata' "$(cat "$STATUS_FILE") $(cat "$BODY_FILE")"
    return
  fi
  KEY_ID=$(body_value '.key_id')
  body_value '.pem' > "$TMP_DIR/catalog-public.pem"

  api_request GET '/api/v1/catalog/manifest' 0
  if ! status_is 200 || ! jq_ok '.payload.schema_version == 2 and (.payload.apps | type == "array" and length > 0) and (.payload.apps[0].versions[0].artifacts[0].architectures | type == "array" and length > 0) and .payload_base64 and .signature.value and .signature.algorithm == "rsa-sha256-pkcs1v15"'; then
    err "$NAME" Manifest 'signed manifest envelope with non-empty catalog' "$(cat "$STATUS_FILE") $(cat "$BODY_FILE")"
    return
  fi
  if [ "$(body_value '.signature.key_id')" != "$KEY_ID" ]; then
    err "$NAME" KeyID 'manifest and public-key key_id match' "$(body_value '.signature.key_id') != $KEY_ID"
    return
  fi

  PAYLOAD_B64=$(body_value '.payload_base64')
  SIGNATURE_B64=$(body_value '.signature.value')
  if ! printf '%s' "$PAYLOAD_B64" | base64 -d > "$TMP_DIR/catalog-payload.json" 2>/dev/null; then
    err "$NAME" PayloadBase64 'valid base64 payload' 'base64 decode failed'
    return
  fi
  if ! printf '%s' "$SIGNATURE_B64" | base64 -d > "$TMP_DIR/catalog-signature.bin" 2>/dev/null; then
    err "$NAME" SignatureBase64 'valid base64 signature' 'base64 decode failed'
    return
  fi
  if ! openssl dgst -sha256 -verify "$TMP_DIR/catalog-public.pem" -signature "$TMP_DIR/catalog-signature.bin" "$TMP_DIR/catalog-payload.json" >/dev/null 2>&1; then
    err "$NAME" SignatureVerification 'valid RSA-SHA256 signature' 'openssl verification failed'
    return
  fi

  cp "$TMP_DIR/catalog-payload.json" "$TMP_DIR/catalog-payload-tampered.json"
  printf ' ' >> "$TMP_DIR/catalog-payload-tampered.json"
  if openssl dgst -sha256 -verify "$TMP_DIR/catalog-public.pem" -signature "$TMP_DIR/catalog-signature.bin" "$TMP_DIR/catalog-payload-tampered.json" >/dev/null 2>&1; then
    err "$NAME" TamperDetection 'tampered payload rejected' 'tampered payload unexpectedly verified'
    return
  fi

  ok "$NAME"
}

test_provisioned_admin() {
  NAME='ProvisionedAdmin'
  ADMIN_EMAIL=${API_TEST_ADMIN_EMAIL:-}
  ADMIN_PASSWORD=${API_TEST_ADMIN_PASSWORD:-}
  if [ -z "$ADMIN_EMAIL" ] || [ -z "$ADMIN_PASSWORD" ]; then
    normal "$NAME"
    return
  fi

  api_request POST '/api/v1/auth/login' 1 '' "{\"email\":\"$ADMIN_EMAIL\",\"password\":\"$ADMIN_PASSWORD\"}"
  ADMIN_TOKEN=$(body_value '.session_token // empty')
  if ! status_is 200 || [ -z "$ADMIN_TOKEN" ] || ! jq_ok '.user.roles | index("admin")'; then
    err "$NAME" Login 'provisioned admin can log in and has admin role' "$(cat "$STATUS_FILE") $(cat "$BODY_FILE")"
    return
  fi

  api_request GET '/api/v1/admin/users?limit=100' 1 "$ADMIN_TOKEN"
  if ! status_is 200 || ! jq_ok '[.users[] | select(.roles | index("admin"))] | length == 1'; then
    err "$NAME" SingleAdmin 'exactly one admin account' "$(cat "$STATUS_FILE") $(cat "$BODY_FILE")"
    return
  fi

  ok "$NAME"
}
