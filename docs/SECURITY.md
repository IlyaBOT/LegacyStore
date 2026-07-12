# Security Model

## Legacy Client

The legacy client must never receive or transmit the primary account password.

Allowed authentication method:

- email
- app-specific legacy password

The app-specific password must be exchanged for short-lived tokens over HTTPS only.

If TLS fails, authentication must be blocked.

No override is allowed for broken TLS during authentication.

## Allowed legacy client scopes

- catalog:read
- downloads:read
- reviews:write
- reviews:like
- profile:read_basic

## Forbidden legacy client scopes

- apps:upload
- apps:edit
- artifacts:upload
- artifacts:edit
- moderation:approve
- moderation:reject
- users:manage
- roles:manage
- admin:any
- profile:change_email
- profile:change_password
- profile:disable_2fa

## Uploads

Software uploads are allowed only through the modern HTTPS web UI.

Uploaded files must go through quarantine, metadata extraction, SHA-256 calculation and moderation.