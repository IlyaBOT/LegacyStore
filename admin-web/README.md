# LegacyStore Web Client

Static web client for browsing the LegacyStore catalog.

The UI is served by nginx on port 8081 and HTTPS port 8443. The same nginx
service proxies `/api/` to the Go backend service, so browser requests can use
relative API URLs.

Authentication, profile, legacy passwords, reviews and moderation screens are
present in the UI. Authenticated actions are intentionally blocked outside
working HTTPS/TLS.
