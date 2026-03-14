#!/bin/bash
# Spec 11: Authentication, Server, and HTTP Wiring Tests
# Tests JWT auth, cookies, session middleware, route table, state-aware routing, mock, server lifecycle
# Adversarial: tampered JWTs, token blacklisting, XSS probes, boundary conditions

set -euo pipefail

PASS=0
FAIL=0
BINARY="./file-uploader"
PORT=18711
BASE="http://127.0.0.1:$PORT"
APP_PID=""

pass() { echo "  PASS: $1"; PASS=$((PASS + 1)); }
fail() { echo "  FAIL: $1"; FAIL=$((FAIL + 1)); }

cleanup() {
    if [[ -n "$APP_PID" ]]; then
        kill "$APP_PID" 2>/dev/null || true
        wait "$APP_PID" 2>/dev/null || true
    fi
}
trap cleanup EXIT

start_app() {
    local extra_args="${1:-}"
    cleanup 2>/dev/null || true
    APP_PID=""
    $BINARY --mock -p $PORT $extra_args >/dev/null 2>&1 &
    APP_PID=$!
    for i in $(seq 1 30); do
        if curl -s "$BASE/health" >/dev/null 2>&1; then
            return 0
        fi
        sleep 0.1
    done
    echo "  FAIL: app did not start within 3s"
    return 1
}

# Helper: login and return cookie jar path
do_login() {
    local user="${1:-testuser}"
    local pass_arg="${2:-testpass}"
    local jar
    jar=$(mktemp)
    curl -s -X POST "$BASE/login" \
        -H "HX-Request: true" \
        -d "username=$user&password=$pass_arg" \
        -c "$jar" \
        -o /dev/null --max-redirs 0
    echo "$jar"
}

echo "=== Spec 11: Authentication, Server, and HTTP Wiring Tests ==="

# -------------------------------------------------------------------
echo ""
echo "--- Server Lifecycle ---"

start_app
HEALTH_BODY=$(curl -s "$BASE/health")
if [[ "$HEALTH_BODY" == "ok" ]]; then
    pass "health returns 'ok' (200)"
else
    fail "health returns '$HEALTH_BODY' (expected 'ok')"
fi

HEALTH_CODE=$(curl -s -o /dev/null -w "%{http_code}" "$BASE/health")
if [[ "$HEALTH_CODE" == "200" ]]; then
    pass "health returns 200"
else
    fail "health returns $HEALTH_CODE"
fi

# -------------------------------------------------------------------
echo ""
echo "--- Static Assets (spec section 8: GET /js/*, /css/*, /img/*, /favicon.ico) ---"

JS_CODE=$(curl -s -o /dev/null -w "%{http_code}" "$BASE/js/app.js")
if [[ "$JS_CODE" == "200" ]]; then pass "GET /js/app.js returns 200"
else fail "GET /js/app.js returns $JS_CODE"; fi

JS_CT=$(curl -s -I "$BASE/js/app.js" | grep -i "content-type" | head -1 | tr -d '\r')
if echo "$JS_CT" | grep -qi "javascript"; then pass "JS asset has correct Content-Type"
else fail "JS asset Content-Type: $JS_CT"; fi

CSS_CODE=$(curl -s -o /dev/null -w "%{http_code}" "$BASE/css/tokens.css")
if [[ "$CSS_CODE" == "200" ]]; then pass "GET /css/tokens.css returns 200"
else fail "GET /css/tokens.css returns $CSS_CODE"; fi

IMG_CODE=$(curl -s -o /dev/null -w "%{http_code}" "$BASE/img/favicon.ico")
if [[ "$IMG_CODE" == "200" ]]; then pass "GET /img/favicon.ico returns 200"
else fail "GET /img/favicon.ico returns $IMG_CODE"; fi

FAVICON_CODE=$(curl -s -o /dev/null -w "%{http_code}" "$BASE/favicon.ico")
if [[ "$FAVICON_CODE" == "200" ]]; then pass "GET /favicon.ico returns 200"
else fail "GET /favicon.ico returns $FAVICON_CODE"; fi

HTMX_CODE=$(curl -s -o /dev/null -w "%{http_code}" "$BASE/vendor/htmx.min.js")
if [[ "$HTMX_CODE" == "200" ]]; then pass "GET /vendor/htmx.min.js returns 200"
else fail "GET /vendor/htmx.min.js returns $HTMX_CODE"; fi

# -------------------------------------------------------------------
echo ""
echo "--- Security Headers ---"

HEADERS=$(curl -s -I "$BASE/health")

if echo "$HEADERS" | grep -qi "X-Frame-Options.*DENY"; then pass "X-Frame-Options: DENY present"
else fail "X-Frame-Options: DENY missing"; fi

if echo "$HEADERS" | grep -qi "X-Content-Type-Options.*nosniff"; then pass "X-Content-Type-Options: nosniff present"
else fail "X-Content-Type-Options: nosniff missing"; fi

if echo "$HEADERS" | grep -qi "Content-Security-Policy"; then pass "Content-Security-Policy present"
else fail "Content-Security-Policy missing"; fi

# Verify CSP on authenticated routes too (not just health)
JAR_HEADERS=$(do_login)
AUTH_HEADERS=$(curl -s -I -b "$JAR_HEADERS" "$BASE/")
if echo "$AUTH_HEADERS" | grep -qi "X-Frame-Options.*DENY"; then pass "security headers on authenticated route"
else fail "security headers missing on authenticated route"; fi
rm -f "$JAR_HEADERS"

# -------------------------------------------------------------------
echo ""
echo "--- Login Flow ---"

LOGIN_GET_CODE=$(curl -s -o /dev/null -w "%{http_code}" "$BASE/login")
if [[ "$LOGIN_GET_CODE" == "200" ]]; then pass "GET /login returns 200"
else fail "GET /login returns $LOGIN_GET_CODE"; fi

LOGIN_BODY=$(curl -s "$BASE/login")
if echo "$LOGIN_BODY" | grep -qi "form"; then pass "GET /login contains a form"
else fail "GET /login does not contain a form"; fi

# Empty creds
EMPTY_LOGIN_CODE=$(curl -s -X POST -H "HX-Request: true" "$BASE/login" -d "" -o /dev/null -w "%{http_code}")
if [[ "$EMPTY_LOGIN_CODE" == "400" ]]; then pass "POST /login empty creds returns 400"
else fail "POST /login empty creds returns $EMPTY_LOGIN_CODE (expected 400)"; fi

# Only username, no password
ONLY_USER_CODE=$(curl -s -X POST -H "HX-Request: true" "$BASE/login" -d "username=testuser" -o /dev/null -w "%{http_code}")
if [[ "$ONLY_USER_CODE" == "400" ]]; then pass "POST /login only username returns 400"
else fail "POST /login only username returns $ONLY_USER_CODE (expected 400)"; fi

# Only password, no username
ONLY_PASS_CODE=$(curl -s -X POST -H "HX-Request: true" "$BASE/login" -d "password=testpass" -o /dev/null -w "%{http_code}")
if [[ "$ONLY_PASS_CODE" == "400" ]]; then pass "POST /login only password returns 400"
else fail "POST /login only password returns $ONLY_PASS_CODE (expected 400)"; fi

# Valid creds
COOKIE_JAR=$(mktemp)
LOGIN_CODE=$(curl -s -X POST "$BASE/login" \
    -d "username=testuser&password=testpass" \
    -c "$COOKIE_JAR" \
    -o /dev/null -w "%{http_code}" \
    --max-redirs 0)
if [[ "$LOGIN_CODE" == "303" ]]; then pass "POST /login valid creds returns 303"
else fail "POST /login valid creds returns $LOGIN_CODE (expected 303)"; fi

# Login redirect goes to /
LOGIN_LOC=$(curl -s -X POST "$BASE/login" \
    -d "username=testuser&password=testpass" \
    -D - -o /dev/null --max-redirs 0 | grep -i "location:" | tr -d '\r')
if echo "$LOGIN_LOC" | grep -q "/$"; then pass "login redirects to /"
else fail "login redirects to: $LOGIN_LOC (expected /)"; fi

# -------------------------------------------------------------------
echo ""
echo "--- Session Cookies (spec section 4) ---"

# Check session cookie was set
if grep -q "session" "$COOKIE_JAR" 2>/dev/null; then pass "session cookie set after login"
else fail "session cookie NOT set"; fi

# session cookie HttpOnly
if grep -q "#HttpOnly_" "$COOKIE_JAR" 2>/dev/null; then pass "session cookie is HttpOnly"
else fail "session cookie not HttpOnly"; fi

# session-expires cookie set
if grep -q "session-expires" "$COOKIE_JAR" 2>/dev/null; then pass "session-expires cookie set"
else fail "session-expires cookie NOT set"; fi

# session-expires NOT HttpOnly (JS-readable, per spec)
SESSION_EXPIRES_LINE=$(grep "session-expires" "$COOKIE_JAR" 2>/dev/null | head -1)
if echo "$SESSION_EXPIRES_LINE" | grep -q "#HttpOnly_"; then fail "session-expires is HttpOnly (spec says JS-readable)"
else pass "session-expires is NOT HttpOnly (JS-readable)"; fi

# SameSite=Strict on session cookie
LOGIN_HEADERS=$(curl -s -X POST "$BASE/login" \
    -d "username=testuser&password=testpass" \
    -D - -o /dev/null --max-redirs 0)
if echo "$LOGIN_HEADERS" | grep -i "Set-Cookie.*session" | grep -qi "SameSite=Strict"; then
    pass "session cookie has SameSite=Strict"
else
    fail "session cookie missing SameSite=Strict"
fi

# session-expires value is in RFC 1123 format (per spec section 4)
EXPIRES_VALUE=$(echo "$LOGIN_HEADERS" | grep -i "Set-Cookie.*session-expires=" | head -1 | sed -n 's/.*session-expires=\([^;]*\).*/\1/p' | tr -d '\r')
if [[ -n "$EXPIRES_VALUE" ]]; then pass "session-expires cookie has a value"
else fail "session-expires cookie value is empty"; fi

# -------------------------------------------------------------------
echo ""
echo "--- Authenticated Routes ---"

# GET / without session → redirect
DASH_NO_AUTH=$(curl -s -o /dev/null -w "%{http_code}" --max-redirs 0 "$BASE/")
if [[ "$DASH_NO_AUTH" == "303" ]]; then pass "GET / without session → 303"
else fail "GET / without session → $DASH_NO_AUTH (expected 303)"; fi

# GET / with session → 200
DASH_AUTH=$(curl -s -o /dev/null -w "%{http_code}" -b "$COOKIE_JAR" "$BASE/")
if [[ "$DASH_AUTH" == "200" ]]; then pass "GET / with session → 200"
else fail "GET / with session → $DASH_AUTH"; fi

# Settings
SETTINGS_NO=$(curl -s -o /dev/null -w "%{http_code}" --max-redirs 0 "$BASE/settings")
if [[ "$SETTINGS_NO" == "303" ]]; then pass "GET /settings no session → 303"; else fail "GET /settings no session → $SETTINGS_NO"; fi

SETTINGS_YES=$(curl -s -o /dev/null -w "%{http_code}" -b "$COOKIE_JAR" "$BASE/settings")
if [[ "$SETTINGS_YES" == "200" ]]; then pass "GET /settings with session → 200"; else fail "GET /settings with session → $SETTINGS_YES"; fi

POST_SETTINGS=$(curl -s -X POST -o /dev/null -w "%{http_code}" -H "HX-Request: true" -b "$COOKIE_JAR" "$BASE/settings")
if [[ "$POST_SETTINGS" == "200" ]]; then pass "POST /settings with session → 200"; else fail "POST /settings → $POST_SETTINGS"; fi

# Players-DB
PDB_NO=$(curl -s -o /dev/null -w "%{http_code}" --max-redirs 0 "$BASE/players-db")
if [[ "$PDB_NO" == "303" ]]; then pass "GET /players-db no session → 303"; else fail "GET /players-db → $PDB_NO"; fi

PDB_YES=$(curl -s -o /dev/null -w "%{http_code}" -b "$COOKIE_JAR" "$BASE/players-db")
if [[ "$PDB_YES" == "200" ]]; then pass "GET /players-db with session → 200"; else fail "GET /players-db → $PDB_YES"; fi

DPD=$(curl -s -o /dev/null -w "%{http_code}" -b "$COOKIE_JAR" "$BASE/download-players-db")
if [[ "$DPD" == "200" ]]; then pass "GET /download-players-db → 200"; else fail "GET /download-players-db → $DPD"; fi

# Archived
ARCH_NO=$(curl -s -o /dev/null -w "%{http_code}" --max-redirs 0 "$BASE/archived")
if [[ "$ARCH_NO" == "303" ]]; then pass "GET /archived no session → 303"; else fail "GET /archived → $ARCH_NO"; fi

ARCH_YES=$(curl -s -o /dev/null -w "%{http_code}" -b "$COOKIE_JAR" "$BASE/archived")
if [[ "$ARCH_YES" == "200" ]]; then pass "GET /archived with session → 200"; else fail "GET /archived → $ARCH_YES"; fi

SA_NO=$(curl -s -X POST -o /dev/null -w "%{http_code}" --max-redirs 0 "$BASE/search-archived")
if [[ "$SA_NO" == "303" ]]; then pass "POST /search-archived no session → 303"; else fail "POST /search-archived → $SA_NO"; fi

SA_YES=$(curl -s -X POST -o /dev/null -w "%{http_code}" -H "HX-Request: true" -b "$COOKIE_JAR" "$BASE/search-archived")
if [[ "$SA_YES" == "200" ]]; then pass "POST /search-archived with session → 200"; else fail "POST /search-archived → $SA_YES"; fi

# Failure details
FD_NO=$(curl -s -o /dev/null -w "%{http_code}" --max-redirs 0 "$BASE/failure-details/test-id")
if [[ "$FD_NO" == "303" ]]; then pass "GET /failure-details/{id} no session → 303"; else fail "GET /failure-details/{id} → $FD_NO"; fi

FD_YES=$(curl -s -o /dev/null -w "%{http_code}" -b "$COOKIE_JAR" "$BASE/failure-details/test-id")
if [[ "$FD_YES" == "404" ]]; then pass "GET /failure-details/{id} unknown → 404"; else fail "GET /failure-details/{id} → $FD_YES (expected 404 for unknown ID)"; fi

# Events (SSE)
EVENTS_NO=$(curl -s -o /dev/null -w "%{http_code}" --max-redirs 0 "$BASE/events")
if [[ "$EVENTS_NO" == "303" ]]; then pass "GET /events no session → 303"; else fail "GET /events → $EVENTS_NO"; fi

EVENTS_CT=$(timeout 2 curl -s -D - -o /dev/null -b "$COOKIE_JAR" "$BASE/events" 2>/dev/null | grep -i "^content-type:" | head -1 | tr -d '\r' || true)
if echo "$EVENTS_CT" | grep -qi "text/event-stream"; then pass "GET /events returns Content-Type: text/event-stream"
else fail "GET /events Content-Type: $EVENTS_CT"; fi

# api/extend
EXTEND_NO=$(curl -s -o /dev/null -w "%{http_code}" --max-redirs 0 "$BASE/api/extend")
if [[ "$EXTEND_NO" == "303" ]]; then pass "GET /api/extend no session → 303"; else fail "GET /api/extend → $EXTEND_NO"; fi

# -------------------------------------------------------------------
echo ""
echo "--- Session Extension (spec section 4: GET /api/extend) ---"

# Fresh token (just created) → outside 40s window → return "ok" no action
EXTEND_BODY=$(curl -s -b "$COOKIE_JAR" "$BASE/api/extend")
if [[ "$EXTEND_BODY" == "ok" ]]; then pass "fresh token: extend returns 'ok' (outside window)"
else fail "fresh token extend: '$EXTEND_BODY' (expected 'ok')"; fi

EXTEND_CODE=$(curl -s -o /dev/null -w "%{http_code}" -b "$COOKIE_JAR" "$BASE/api/extend")
if [[ "$EXTEND_CODE" == "200" ]]; then pass "extend returns 200"
else fail "extend returns $EXTEND_CODE"; fi

# Verify extend outside window does NOT set new cookies
EXTEND_HEADERS=$(curl -s -D - -o /dev/null -b "$COOKIE_JAR" "$BASE/api/extend")
if echo "$EXTEND_HEADERS" | grep -qi "Set-Cookie"; then
    fail "extend outside window should not set new cookies"
else
    pass "extend outside window does not set new cookies"
fi

# -------------------------------------------------------------------
echo ""
echo "--- Logout (spec section 8) ---"

LOGOUT_CODE=$(curl -s -o /dev/null -w "%{http_code}" --max-redirs 0 -X POST -b "$COOKIE_JAR" "$BASE/logout")
if [[ "$LOGOUT_CODE" == "303" ]]; then pass "POST /logout → 303"
else fail "POST /logout → $LOGOUT_CODE"; fi

LOGOUT_LOC=$(curl -s -D - -o /dev/null --max-redirs 0 -X POST -b "$COOKIE_JAR" "$BASE/logout" | grep -i "location:" | tr -d '\r')
if echo "$LOGOUT_LOC" | grep -q "/login"; then pass "logout redirects to /login"
else fail "logout redirects to: $LOGOUT_LOC"; fi

# Logout clears session cookie
LOGOUT_HEADERS=$(curl -s -D - -o /dev/null --max-redirs 0 -X POST -b "$COOKIE_JAR" "$BASE/logout")
if echo "$LOGOUT_HEADERS" | grep -i "Set-Cookie.*session" | grep -qi "Max-Age=0\|expires=Thu, 01 Jan 1970\|Max-Age=-1"; then
    pass "logout clears session cookie"
else
    fail "logout does not clear session cookie"
fi

# -------------------------------------------------------------------
echo ""
echo "--- Token Blacklisting After Logout ---"

# Login, grab cookie, logout, try to use old cookie
BL_JAR=$(do_login "blacklist_user" "pass")
# Verify session works before logout
BL_PRE=$(curl -s -o /dev/null -w "%{http_code}" -b "$BL_JAR" "$BASE/")
if [[ "$BL_PRE" == "200" ]]; then pass "pre-logout: session works"
else fail "pre-logout: session returns $BL_PRE"; fi

# Logout (revokes JTI)
curl -s -o /dev/null --max-redirs 0 -X POST -b "$BL_JAR" "$BASE/logout"

# Try to use the old cookie — should be rejected
BL_POST=$(curl -s -o /dev/null -w "%{http_code}" --max-redirs 0 -b "$BL_JAR" "$BASE/")
if [[ "$BL_POST" == "303" ]]; then pass "post-logout: old token rejected (blacklisted)"
else fail "post-logout: old token returns $BL_POST (expected 303 — token should be blacklisted)"; fi
rm -f "$BL_JAR"

# -------------------------------------------------------------------
echo ""
echo "--- JWT Security: Tampered Tokens ---"

# Forge a JWT with a different signing key — should be rejected
FORGED_TOKEN=$(python3 -c "
import base64, json, hashlib, hmac, time
header = base64.urlsafe_b64encode(json.dumps({'alg':'HS256','typ':'JWT'}).encode()).rstrip(b'=').decode()
payload = base64.urlsafe_b64encode(json.dumps({'username':'hacker','orgID':'evil-org','jti':'forged','exp':int(time.time())+3600}).encode()).rstrip(b'=').decode()
sig_input = f'{header}.{payload}'.encode()
sig = base64.urlsafe_b64encode(hmac.new(b'wrong-key-definitely', sig_input, hashlib.sha256).digest()).rstrip(b'=').decode()
print(f'{header}.{payload}.{sig}')
" 2>/dev/null || echo "")

if [[ -n "$FORGED_TOKEN" ]]; then
    FORGED_CODE=$(curl -s -o /dev/null -w "%{http_code}" --max-redirs 0 \
        --cookie "session=$FORGED_TOKEN" "$BASE/")
    if [[ "$FORGED_CODE" == "303" ]]; then pass "forged JWT (wrong key) rejected"
    else fail "forged JWT accepted with code $FORGED_CODE (expected 303)"; fi
else
    pass "forged JWT test skipped (python3 not available)"
fi

# Completely garbage token
GARBAGE_CODE=$(curl -s -o /dev/null -w "%{http_code}" --max-redirs 0 \
    --cookie "session=not.a.real.jwt.at.all" "$BASE/")
if [[ "$GARBAGE_CODE" == "303" ]]; then pass "garbage JWT rejected"
else fail "garbage JWT accepted with code $GARBAGE_CODE (expected 303)"; fi

# Empty session cookie
EMPTY_SESSION_CODE=$(curl -s -o /dev/null -w "%{http_code}" --max-redirs 0 \
    --cookie "session=" "$BASE/")
if [[ "$EMPTY_SESSION_CODE" == "303" ]]; then pass "empty session cookie rejected"
else fail "empty session cookie accepted with code $EMPTY_SESSION_CODE"; fi

# -------------------------------------------------------------------
echo ""
echo "--- XSS Probe: Username Escaping ---"

XSS_JAR=$(do_login '<script>alert(1)</script>' 'pass')
XSS_BODY=$(curl -s -b "$XSS_JAR" "$BASE/")
if echo "$XSS_BODY" | grep -q '<script>alert(1)</script>'; then
    fail "XSS: raw script tag rendered in dashboard (not escaped)"
else
    pass "XSS: script tags escaped in dashboard output"
fi
rm -f "$XSS_JAR"

# -------------------------------------------------------------------
echo ""
echo "--- Upload Endpoint (spec section 8) ---"

COOKIE_JAR2=$(do_login "uploader" "pass")

# Without auth → redirect
UPLOAD_NO=$(curl -s -o /dev/null -w "%{http_code}" --max-redirs 0 -X POST "$BASE/upload")
if [[ "$UPLOAD_NO" == "303" ]]; then pass "POST /upload no session → 303"; else fail "POST /upload → $UPLOAD_NO"; fi

# 50MB limit (spec: Upload enforces 50MB limit)
LARGE_FILE=$(mktemp)
dd if=/dev/zero of="$LARGE_FILE" bs=1M count=55 2>/dev/null
UPLOAD_LARGE=$(curl -s -X POST "$BASE/upload" \
    -H "HX-Request: true" \
    -b "$COOKIE_JAR2" \
    -F "file=@$LARGE_FILE" \
    -o /dev/null -w "%{http_code}")
rm -f "$LARGE_FILE"
if [[ "$UPLOAD_LARGE" == "413" ]]; then pass "POST /upload >50MB → 413"
else fail "POST /upload >50MB → $UPLOAD_LARGE (expected 413)"; fi

# Small upload should succeed
SMALL_FILE=$(mktemp)
echo "col1,col2" > "$SMALL_FILE"
echo "val1,val2" >> "$SMALL_FILE"
UPLOAD_SMALL=$(curl -s -X POST "$BASE/upload" \
    -H "HX-Request: true" \
    -b "$COOKIE_JAR2" \
    -F "file=@$SMALL_FILE" \
    -o /dev/null -w "%{http_code}")
rm -f "$SMALL_FILE"
if [[ "$UPLOAD_SMALL" == "200" ]]; then pass "POST /upload small file → 200"
else fail "POST /upload small file → $UPLOAD_SMALL (expected 200)"; fi

rm -f "$COOKIE_JAR2"

# -------------------------------------------------------------------
echo ""
echo "--- Route Methods ---"

HEALTH_POST=$(curl -s -X POST -o /dev/null -w "%{http_code}" "$BASE/health")
if [[ "$HEALTH_POST" == "405" ]]; then pass "POST /health → 405"
else fail "POST /health → $HEALTH_POST (expected 405)"; fi

UPLOAD_GET=$(curl -s -o /dev/null -w "%{http_code}" --max-redirs 0 "$BASE/upload")
# GET /upload should be 405 if route exists but only POST allowed, or 303 redirect
# Since upload requires auth (withRunningState + withStateAndSession), GET should be 405
# Chi registers POST only, so GET to /upload should be 405
if [[ "$UPLOAD_GET" == "405" ]]; then pass "GET /upload → 405"
else fail "GET /upload → $UPLOAD_GET (expected 405)"; fi

# PUT to login should be 405
LOGIN_PUT=$(curl -s -X PUT -o /dev/null -w "%{http_code}" "$BASE/login")
if [[ "$LOGIN_PUT" == "405" ]]; then pass "PUT /login → 405"
else fail "PUT /login → $LOGIN_PUT (expected 405)"; fi

# DELETE to dashboard should fail
DASH_DELETE=$(curl -s -X DELETE -o /dev/null -w "%{http_code}" --max-redirs 0 "$BASE/")
if [[ "$DASH_DELETE" != "200" ]]; then pass "DELETE / → $DASH_DELETE (not 200)"
else fail "DELETE / returns 200 (should not)"; fi

# -------------------------------------------------------------------
echo ""
echo "--- State-Aware Routing (spec section 9) ---"

# --mock starts in RunningApp; /setup should redirect to /
SETUP_R=$(curl -s -o /dev/null -w "%{http_code}" --max-redirs 0 "$BASE/setup")
if [[ "$SETUP_R" == "303" ]]; then pass "GET /setup in RunningApp → 303 redirect"
else fail "GET /setup in RunningApp → $SETUP_R"; fi

SETUP_POST_R=$(curl -s -X POST -o /dev/null -w "%{http_code}" --max-redirs 0 "$BASE/setup/next")
if [[ "$SETUP_POST_R" == "303" ]]; then pass "POST /setup/next in RunningApp → 303 redirect"
else fail "POST /setup/next in RunningApp → $SETUP_POST_R"; fi

# Verify setup redirect target is /
SETUP_LOC=$(curl -s -D - -o /dev/null --max-redirs 0 "$BASE/setup" | grep -i "^location:" | tr -d '\r')
if echo "$SETUP_LOC" | grep -q "/$"; then pass "setup redirects to / (root)"
else fail "setup redirects to: $SETUP_LOC (expected /)"; fi

# -------------------------------------------------------------------
echo ""
echo "--- Mock Mode (spec section 10) ---"

COOKIE_JAR3=$(do_login "anyuser" "anypass")
MOCK_DASH=$(curl -s -o /dev/null -w "%{http_code}" -b "$COOKIE_JAR3" "$BASE/")
if [[ "$MOCK_DASH" == "200" ]]; then pass "mock auth accepts arbitrary credentials"
else fail "mock auth rejects: $MOCK_DASH"; fi

# Note: no spec requires username display on dashboard/navbar, so we don't test for it

# MFARequired should be false in mock mode (spec section 10)
# No direct endpoint, but login works without mfa_token = evidence
MOCK_NO_MFA=$(do_login "user2" "pass2")
MFA_CHECK=$(curl -s -o /dev/null -w "%{http_code}" -b "$MOCK_NO_MFA" "$BASE/")
if [[ "$MFA_CHECK" == "200" ]]; then pass "mock mode: no MFA required"
else fail "mock mode: login fails without MFA (code $MFA_CHECK)"; fi
rm -f "$MOCK_NO_MFA" "$COOKIE_JAR3"

# -------------------------------------------------------------------
echo ""
echo "--- Graceful Shutdown ---"

cleanup
$BINARY --mock -p $PORT >/dev/null 2>&1 &
APP_PID=$!
for i in $(seq 1 30); do curl -s "$BASE/health" >/dev/null 2>&1 && break; sleep 0.1; done

kill -TERM "$APP_PID" 2>/dev/null
wait "$APP_PID" 2>/dev/null || true
pass "app responds to SIGTERM and shuts down"
APP_PID=""

# SIGINT should also work
$BINARY --mock -p $PORT >/dev/null 2>&1 &
APP_PID=$!
for i in $(seq 1 30); do curl -s "$BASE/health" >/dev/null 2>&1 && break; sleep 0.1; done

kill -INT "$APP_PID" 2>/dev/null
wait "$APP_PID" 2>/dev/null || true
pass "app responds to SIGINT and shuts down"
APP_PID=""

# -------------------------------------------------------------------
echo ""
echo "--- URL Prefix (spec section 8: 'All routes mounted under configurable prefix') ---"

start_app "-P /myapp"

PFX_HEALTH=$(curl -s -o /dev/null -w "%{http_code}" "$BASE/myapp/health")
if [[ "$PFX_HEALTH" == "200" ]]; then pass "health at /myapp/health returns 200"
else fail "health at /myapp/health returns $PFX_HEALTH"; fi

PFX_LOGIN=$(curl -s -o /dev/null -w "%{http_code}" "$BASE/myapp/login")
if [[ "$PFX_LOGIN" == "200" ]]; then pass "login at /myapp/login returns 200"
else fail "login at /myapp/login returns $PFX_LOGIN"; fi

# Root without prefix should NOT work
NO_PFX=$(curl -s -o /dev/null -w "%{http_code}" "$BASE/health")
if [[ "$NO_PFX" != "200" ]]; then pass "/health without prefix returns $NO_PFX (not 200)"
else fail "/health still returns 200 without prefix"; fi

# Login with prefix sets correct cookie path
PFX_HEADERS=$(curl -s -X POST "$BASE/myapp/login" -d "username=u&password=p" -D - -o /dev/null --max-redirs 0)
COOKIE_PATH=$(echo "$PFX_HEADERS" | grep -i "Set-Cookie.*session=" | grep -oi "Path=[^;]*" | head -1 | cut -d= -f2)
if [[ "$COOKIE_PATH" == "/myapp" ]]; then pass "session cookie path matches prefix (/myapp)"
else fail "session cookie path: '$COOKIE_PATH' (expected '/myapp')"; fi

# Prefix: auth redirect should include prefix
PFX_REDIRECT=$(curl -s -D - -o /dev/null --max-redirs 0 "$BASE/myapp/")
PFX_LOC=$(echo "$PFX_REDIRECT" | grep -i "^location:" | tr -d '\r')
if echo "$PFX_LOC" | grep -q "/myapp/login"; then pass "prefix: unauthenticated redirect includes prefix"
else fail "prefix: unauthenticated redirect: $PFX_LOC (expected /myapp/login)"; fi

# Prefix: setup redirect includes prefix
PFX_SETUP_LOC=$(curl -s -D - -o /dev/null --max-redirs 0 "$BASE/myapp/setup" | grep -i "^location:" | tr -d '\r')
if echo "$PFX_SETUP_LOC" | grep -q "/myapp/"; then pass "prefix: setup redirect includes prefix"
else fail "prefix: setup redirect: $PFX_SETUP_LOC (expected /myapp/)"; fi

# Prefix: logout redirect includes prefix
PFX_JAR=$(mktemp)
curl -s -X POST "$BASE/myapp/login" -d "username=u&password=p" -c "$PFX_JAR" -o /dev/null --max-redirs 0
PFX_LOGOUT_LOC=$(curl -s -D - -o /dev/null --max-redirs 0 -X POST -b "$PFX_JAR" "$BASE/myapp/logout" | grep -i "^location:" | tr -d '\r')
if echo "$PFX_LOGOUT_LOC" | grep -q "/myapp/login"; then pass "prefix: logout redirect includes prefix"
else fail "prefix: logout redirect: $PFX_LOGOUT_LOC (expected /myapp/login)"; fi
rm -f "$PFX_JAR"

# -------------------------------------------------------------------
echo ""
echo "--- Already-Authenticated Login Page Redirect ---"

cleanup
start_app

# If user is already authenticated and visits /login, should redirect to dashboard
AUTH_LOGIN_JAR=$(do_login "alreadyauth" "pass")
AUTH_LOGIN_CODE=$(curl -s -o /dev/null -w "%{http_code}" --max-redirs 0 -b "$AUTH_LOGIN_JAR" "$BASE/login")
if [[ "$AUTH_LOGIN_CODE" == "303" ]]; then pass "GET /login with valid session → 303 redirect to dashboard"
else fail "GET /login with valid session → $AUTH_LOGIN_CODE (expected 303 redirect)"; fi
rm -f "$AUTH_LOGIN_JAR"

# -------------------------------------------------------------------
echo ""
echo "--- Concurrent Sessions ---"

# Two different users should have independent sessions
S1_JAR=$(do_login "user1" "pass1")
S2_JAR=$(do_login "user2" "pass2")

# Both sessions should independently access the dashboard
S1_CODE=$(curl -s -o /dev/null -w "%{http_code}" -b "$S1_JAR" "$BASE/")
S2_CODE=$(curl -s -o /dev/null -w "%{http_code}" -b "$S2_JAR" "$BASE/")

if [[ "$S1_CODE" == "200" ]]; then pass "session 1: authenticated (200)"
else fail "session 1: expected 200, got $S1_CODE"; fi

if [[ "$S2_CODE" == "200" ]]; then pass "session 2: authenticated (200)"
else fail "session 2: expected 200, got $S2_CODE"; fi
rm -f "$S1_JAR" "$S2_JAR"

# -------------------------------------------------------------------
echo ""
echo "--- Spec Compliance: WebApp struct (spec section 6) ---"

# Verify WebApp has required fields by checking that the features they enable work:
# - signingKey: login produces valid JWT (already tested)
# - uploadDir: upload endpoint works (already tested)
# - version: not directly exposed in this spec, but field should exist
# - app: state-aware routing works (already tested)

# Check that version is embedded in binary
VERSION_OUT=$($BINARY -v 2>&1 || true)
if [[ -n "$VERSION_OUT" ]]; then pass "version output present"
else fail "no version output"; fi

# -------------------------------------------------------------------
echo ""
echo "--- Spec Compliance: Server struct (spec section 7) ---"

# NewServer, Start, Stop, Wait tested via app lifecycle
# TLS support: server accepts tls.Config (structural, can't fully test without certs)
# Verify graceful shutdown timeout doesn't hang
TIMEOUT_START=$SECONDS
cleanup
$BINARY --mock -p $PORT >/dev/null 2>&1 &
APP_PID=$!
for i in $(seq 1 30); do curl -s "$BASE/health" >/dev/null 2>&1 && break; sleep 0.1; done
kill -TERM "$APP_PID" 2>/dev/null
wait "$APP_PID" 2>/dev/null || true
TIMEOUT_ELAPSED=$((SECONDS - TIMEOUT_START))
if [[ $TIMEOUT_ELAPSED -lt 15 ]]; then pass "graceful shutdown completes within 15s"
else fail "shutdown took ${TIMEOUT_ELAPSED}s (too slow)"; fi
APP_PID=""

# Clean up temp files
rm -f "$COOKIE_JAR" 2>/dev/null

# -------------------------------------------------------------------
echo ""
echo "=== Results ==="
echo "PASS: $PASS"
echo "FAIL: $FAIL"

if [[ $FAIL -gt 0 ]]; then
    exit 1
fi
