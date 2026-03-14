#!/usr/bin/env bash
# Spec 14: Archive, Settings, Players DB, Login Pages
# Tests for archive filters, settings forms, players DB, login/logout
set -euo pipefail

PORT=8199
BASE="http://localhost:$PORT"
APP_PID=""
PASS_COUNT=0
FAIL_COUNT=0

cleanup() {
  if [ -n "$APP_PID" ] && kill -0 "$APP_PID" 2>/dev/null; then
    kill "$APP_PID" 2>/dev/null || true
    wait "$APP_PID" 2>/dev/null || true
  fi
}
trap cleanup EXIT

check() {
  local name="$1" test="$2" detail="${3:-}"
  if eval "$test" >/dev/null 2>&1; then
    echo "  PASS: $name"
    PASS_COUNT=$((PASS_COUNT + 1))
  else
    echo "  FAIL: $name -- $detail"
    FAIL_COUNT=$((FAIL_COUNT + 1))
  fi
}

# --- Start the app ---
./file-uploader --mock -p "$PORT" &
APP_PID=$!
sleep 1

# Wait for health
for i in $(seq 1 30); do
  if curl -s -o /dev/null "$BASE/login" 2>/dev/null; then break; fi
  sleep 0.5
done

# --- Login helper ---
login_cookies() {
  local jar
  jar=$(mktemp)
  curl -s -o /dev/null -c "$jar" -X POST \
    -H "HX-Request: true" \
    -d "username=admin&password=admin" \
    "$BASE/login" 2>/dev/null
  echo "$jar"
}

auth_get() {
  local jar="$1" url="$2"
  curl -s -b "$jar" "$url" 2>/dev/null
}

auth_get_code() {
  local jar="$1" url="$2"
  curl -s -o /dev/null -w "%{http_code}" -b "$jar" "$url" 2>/dev/null
}

auth_post() {
  local jar="$1" url="$2"
  shift 2
  curl -s -b "$jar" -X POST -H "HX-Request: true" "$@" "$url" 2>/dev/null
}

auth_post_code() {
  local jar="$1" url="$2"
  shift 2
  curl -s -o /dev/null -w "%{http_code}" -b "$jar" -X POST -H "HX-Request: true" "$@" "$url" 2>/dev/null
}

echo ""
echo "=== Spec 14: Archive, Settings, Players DB, Login ==="

# Pre-create cookie jars BEFORE the login tests that consume rate-limit slots.
# The login rate limiter allows 5 requests/minute; creating jars first avoids exhaustion.
COOKIE_JAR=$(login_cookies)
COOKIE_JAR2=$(login_cookies)

# ==========================================
# Login tests
# ==========================================
echo ""
echo "--- Login Page ---"

LOGIN_PAGE=$(curl -s "$BASE/login")
check "Login page renders" 'echo "$LOGIN_PAGE" | grep -q "Sign In"' ""
check "Login has username field" 'echo "$LOGIN_PAGE" | grep -q "name=\"username\""' ""
check "Login has password field" 'echo "$LOGIN_PAGE" | grep -q "name=\"password\""' ""
check "Login uses hx-post for form" 'echo "$LOGIN_PAGE" | grep -q "hx-post"' ""
check "Login form targets #login-form" 'echo "$LOGIN_PAGE" | grep -q "hx-target=\"#login-form\""' ""

# MFA conditional - mock returns false so MFA field should NOT be present
check "Login MFA field hidden when not required" '! echo "$LOGIN_PAGE" | grep -q "name=\"mfa_token\""' \
  "MFA field should not appear when MFARequired returns false"

# Login success via htmx POST (requires HX-Request header per spec)
LOGIN_RESP_HEADERS=$(curl -s -D - -o /dev/null -X POST \
  -H "HX-Request: true" \
  -d "username=admin&password=admin" \
  "$BASE/login" 2>/dev/null)
check "Login htmx success returns HX-Redirect header" 'echo "$LOGIN_RESP_HEADERS" | grep -qi "HX-Redirect"' \
  "$(echo "$LOGIN_RESP_HEADERS" | head -5)"
check "Login htmx success HX-Redirect points to /" 'echo "$LOGIN_RESP_HEADERS" | grep -i "HX-Redirect" | grep -q "/"' ""
check "Login success sets session cookie" 'echo "$LOGIN_RESP_HEADERS" | grep -qi "Set-Cookie.*session="' ""
check "Login success sets session-expires cookie" 'echo "$LOGIN_RESP_HEADERS" | grep -qi "Set-Cookie.*session-expires="' ""

# Login session cookie is HttpOnly
check "Login session cookie is HttpOnly" 'echo "$LOGIN_RESP_HEADERS" | grep -i "Set-Cookie" | grep -i "session=" | grep -v "session-expires" | grep -qi "HttpOnly"' ""

# Login without htmx header falls back to 303 redirect
LOGIN_NOHX_CODE=$(curl -s -o /dev/null -w "%{http_code}" -X POST \
  -d "username=admin&password=admin" \
  "$BASE/login" 2>/dev/null)
check "Login non-htmx POST returns 303 redirect" '[ "$LOGIN_NOHX_CODE" = "303" ]' "got $LOGIN_NOHX_CODE"

# Login failure — empty fields (single call to conserve rate limit slots)
LOGIN_EMPTY_RAW=$(curl -s -w "\n%{http_code}" -H "HX-Request: true" -X POST -d "username=&password=" "$BASE/login" 2>/dev/null)
LOGIN_EMPTY_CODE=$(echo "$LOGIN_EMPTY_RAW" | tail -1)
LOGIN_EMPTY_RESP=$(echo "$LOGIN_EMPTY_RAW" | sed '$d')
check "Login empty fields returns 400" '[ "$LOGIN_EMPTY_CODE" = "400" ]' "got $LOGIN_EMPTY_CODE"
check "Login empty fields shows error" 'echo "$LOGIN_EMPTY_RESP" | grep -qi "required\|error"' ""

# Already logged in — redirect to dashboard
REDIR_CODE=$(curl -s -o /dev/null -w "%{http_code}" -b "$COOKIE_JAR" "$BASE/login" 2>/dev/null)
check "Login with valid session redirects (303)" '[ "$REDIR_CODE" = "303" ]' "got $REDIR_CODE"

# Password field is type=password
check "Login password field is type password" 'echo "$LOGIN_PAGE" | grep -q "type=\"password\""' ""

# ==========================================
# Logout tests
# ==========================================
echo ""
echo "--- Logout ---"

# Logout is POST per code review fix
LOGOUT_RESP=$(curl -s -D - -o /dev/null -X POST -b "$COOKIE_JAR2" "$BASE/logout" 2>/dev/null)
LOGOUT_CODE=$(echo "$LOGOUT_RESP" | head -1 | grep -o '[0-9]\{3\}')
check "Logout POST returns 303 redirect" '[ "$LOGOUT_CODE" = "303" ]' "got $LOGOUT_CODE"
check "Logout redirects to /login" 'echo "$LOGOUT_RESP" | grep -qi "Location:.*login"' ""
# Check cookies are cleared (Max-Age=0 or Expires in past)
check "Logout clears session cookie" 'echo "$LOGOUT_RESP" | grep -i "Set-Cookie" | grep -i "session" | grep -qiE "Max-Age=0|expires=.*1970"' ""

# GET /logout should fail (not a valid method)
LOGOUT_GET_CODE=$(curl -s -o /dev/null -w "%{http_code}" "$BASE/logout" 2>/dev/null)
check "GET /logout returns 405 (method not allowed)" '[ "$LOGOUT_GET_CODE" = "405" ]' "got $LOGOUT_GET_CODE"

# ==========================================
# Archive page tests
# ==========================================
echo ""
echo "--- Archive Page ---"

# Archive page loads (reuse COOKIE_JAR from top to avoid rate limiter)
ARCHIVE_PAGE=$(auth_get "$COOKIE_JAR" "$BASE/archived")
check "Archive page renders" 'echo "$ARCHIVE_PAGE" | grep -qi "Archived\|archived"' ""
check "Archive has status dropdown" 'echo "$ARCHIVE_PAGE" | grep -q "name=\"status\""' ""
check "Archive has CSV type dropdown" 'echo "$ARCHIVE_PAGE" | grep -q "name=\"csv_type\""' ""
check "Archive has search input" 'echo "$ARCHIVE_PAGE" | grep -q "name=\"search\""' ""
check "Archive has hx-post for search" 'echo "$ARCHIVE_PAGE" | grep -q "hx-post"' ""
check "Archive has hx-trigger with debounce" 'echo "$ARCHIVE_PAGE" | grep -q "delay:300ms"' ""
check "Archive target is archive-results" 'echo "$ARCHIVE_PAGE" | grep -q "archive-results"' ""

# Archive has status filter options
check "Archive status filter has All option" 'echo "$ARCHIVE_PAGE" | grep -q ">All<"' ""
check "Archive status filter has Success option" 'echo "$ARCHIVE_PAGE" | grep -qi "value=\"success\""' ""
check "Archive status filter has Failure option" 'echo "$ARCHIVE_PAGE" | grep -qi "value=\"failure\""' ""

# CSV type dropdown should have all 10 types
for slug in bets players casino-players bonus casino casino-par-sheet complaints demographic deposits-withdrawals responsible-gaming; do
  check "Archive type filter has $slug" 'echo "$ARCHIVE_PAGE" | grep -q "value=\"'"$slug"'\""' ""
done

# Empty archive state (mock returns no files)
check "Archive shows empty state when no files" 'echo "$ARCHIVE_PAGE" | grep -qi "no archived\|empty"' ""

# Spec compliance: hx-trigger format
check "Archive hx-trigger includes change" 'echo "$ARCHIVE_PAGE" | grep "hx-trigger" | grep -q "change"' ""
check "Archive hx-trigger includes keyup changed" 'echo "$ARCHIVE_PAGE" | grep "hx-trigger" | grep -q "keyup changed"' ""
check "Archive hx-trigger has 300ms delay" 'echo "$ARCHIVE_PAGE" | grep "hx-trigger" | grep -q "delay:300ms"' ""

# Table column headers exist in template (verified via templ source since mock is empty)
# Check the ArchiveResultsTable templ source for required columns
TEMPL_FILE="internal/server/pages/placeholders.templ"
check "Template has File column" 'grep -q ">File<" "$TEMPL_FILE"' ""
check "Template has CSV Type column" 'grep -q ">CSV Type<" "$TEMPL_FILE"' ""
check "Template has Status column" 'grep -q ">Status<" "$TEMPL_FILE"' ""
check "Template has Uploaded By column" 'grep -q ">Uploaded By<" "$TEMPL_FILE"' ""
check "Template has Processed At column" 'grep -q ">Processed At<" "$TEMPL_FILE"' ""
check "Template has Uploaded At column" 'grep -q ">Uploaded At<" "$TEMPL_FILE"' ""
check "Template has Failure Phase column" 'grep -q ">Failure Phase<" "$TEMPL_FILE"' ""

# Failure rows should be clickable per spec
check "Template failure rows have hx-get for details" 'grep -q "hx-get.*failure-details" "$TEMPL_FILE"' ""
check "Template failure rows use modal" 'grep -q "failure-modal" "$TEMPL_FILE"' ""

# Success/failure row styling
check "Template has row-success class" 'grep -q "row-success" "$TEMPL_FILE"' ""
check "Template has row-failure class" 'grep -q "row-failure" "$TEMPL_FILE"' ""

# Auth required
ARCHIVE_NOAUTH=$(curl -s -o /dev/null -w "%{http_code}" "$BASE/archived" 2>/dev/null)
check "Archive without auth redirects (303)" '[ "$ARCHIVE_NOAUTH" = "303" ]' "got $ARCHIVE_NOAUTH"

# ==========================================
# Search archived endpoint
# ==========================================
echo ""
echo "--- Search Archived ---"

SEARCH_CODE=$(auth_post_code "$COOKIE_JAR" "$BASE/search-archived" -d "status=&csv_type=&search=")
check "POST /search-archived returns 200" '[ "$SEARCH_CODE" = "200" ]' "got $SEARCH_CODE"

SEARCH_RESP=$(auth_post "$COOKIE_JAR" "$BASE/search-archived" -d "status=success&csv_type=&search=")
check "Search with status filter returns HTML" 'echo "$SEARCH_RESP" | grep -qiE "table|empty|archived|No "' ""

SEARCH_RESP2=$(auth_post "$COOKIE_JAR" "$BASE/search-archived" -d "status=&csv_type=players&search=")
check "Search with type filter returns HTML" 'echo "$SEARCH_RESP2" | grep -qiE "table|empty|archived|No "' ""

SEARCH_RESP3=$(auth_post "$COOKIE_JAR" "$BASE/search-archived" -d "status=&csv_type=&search=test")
check "Search with text filter returns HTML" 'echo "$SEARCH_RESP3" | grep -qiE "table|empty|archived|No "' ""

SEARCH_NOAUTH=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$BASE/search-archived" 2>/dev/null)
check "Search archived without auth redirects (303)" '[ "$SEARCH_NOAUTH" = "303" ]' "got $SEARCH_NOAUTH"

# ==========================================
# Failure details endpoint
# ==========================================
echo ""
echo "--- Failure Details ---"

# BUG: placeholderRunningApp.GetFinishedDetails returns nil,nil instead of nil,error
# This causes handler to return 200 instead of 404 for unknown records
FAIL_CODE=$(auth_get_code "$COOKIE_JAR" "$BASE/failure-details/nonexistent-id")
check "Failure details for unknown ID returns 404" '[ "$FAIL_CODE" = "404" ]' \
  "got $FAIL_CODE (BUG: placeholderRunningApp.GetFinishedDetails returns nil,nil)"

FAIL_NOAUTH=$(curl -s -o /dev/null -w "%{http_code}" "$BASE/failure-details/test" 2>/dev/null)
check "Failure details without auth redirects (303)" '[ "$FAIL_NOAUTH" = "303" ]' "got $FAIL_NOAUTH"

# ==========================================
# Settings page tests
# ==========================================
echo ""
echo "--- Settings Page ---"

SETTINGS_PAGE=$(auth_get "$COOKIE_JAR" "$BASE/settings")
check "Settings page renders" 'echo "$SETTINGS_PAGE" | grep -qi "settings"' ""

# Four sections per spec
check "Settings has API Endpoint section" 'echo "$SETTINGS_PAGE" | grep -qi "API Endpoint"' ""
check "Settings has Service Credentials section" 'echo "$SETTINGS_PAGE" | grep -qi "Service Credentials"' ""
check "Settings has Player ID Hasher section" 'echo "$SETTINGS_PAGE" | grep -qi "Player ID Hasher"' ""
check "Settings has Use Players DB section" 'echo "$SETTINGS_PAGE" | grep -qi "Use Players DB"' ""

# API Endpoint is read-only (disabled inputs)
check "Settings API endpoint fields are disabled" 'echo "$SETTINGS_PAGE" | grep -q "disabled"' ""

# Hash algorithm is read-only and shows argon2
check "Settings shows argon2 hash algorithm" 'echo "$SETTINGS_PAGE" | grep -q "argon2"' ""

# Pepper input has minlength
check "Settings pepper has minlength=5" 'echo "$SETTINGS_PAGE" | grep -q "minlength=\"5\""' ""

# Registration code is a separate form (htmx POST to /settings/registration)
check "Settings has registration htmx form" 'echo "$SETTINGS_PAGE" | grep -q "hx-post.*registration"' ""

# Main form uses htmx POST to /settings
check "Settings main form uses hx-post" 'echo "$SETTINGS_PAGE" | grep -q "hx-post"' ""
check "Settings main form targets /settings" 'echo "$SETTINGS_PAGE" | grep -q "hx-post=\"/settings\""' ""

# Use Players DB radio buttons
check "Settings has use_players_db radio" 'echo "$SETTINGS_PAGE" | grep -q "name=\"use_players_db\""' ""
check "Settings has Yes radio option (true)" 'echo "$SETTINGS_PAGE" | grep -q "value=\"true\""' ""
check "Settings has No radio option (false)" 'echo "$SETTINGS_PAGE" | grep -q "value=\"false\""' ""
check "Settings radio uses input type=radio" 'echo "$SETTINGS_PAGE" | grep -q "type=\"radio\""' ""

# Auth required
SETTINGS_NOAUTH=$(curl -s -o /dev/null -w "%{http_code}" "$BASE/settings" 2>/dev/null)
check "Settings without auth redirects (303)" '[ "$SETTINGS_NOAUTH" = "303" ]' "got $SETTINGS_NOAUTH"

# ==========================================
# Settings POST (validation)
# ==========================================
echo ""
echo "--- Settings POST ---"

# Post with short pepper (< 5 chars) - should show validation error
SETTINGS_SHORT=$(auth_post "$COOKIE_JAR" "$BASE/settings" -d "pepper=ab&use_players_db=false")
check "Settings short pepper shows validation error" 'echo "$SETTINGS_SHORT" | grep -qi "error\|danger\|too short\|minimum\|at least"' \
  "Response: $(echo "$SETTINGS_SHORT" | head -3)"

# Post with valid data - should succeed
SETTINGS_VALID=$(auth_post "$COOKIE_JAR" "$BASE/settings" -d "pepper=validpepper&use_players_db=true")
check "Settings valid data shows success" 'echo "$SETTINGS_VALID" | grep -qi "success\|saved\|updated"' \
  "Response: $(echo "$SETTINGS_VALID" | head -3)"

# POST settings without auth
SETTINGS_POST_NOAUTH=$(curl -s -o /dev/null -w "%{http_code}" -X POST -d "pepper=test1&use_players_db=true" "$BASE/settings" 2>/dev/null)
check "Settings POST without auth redirects (303)" '[ "$SETTINGS_POST_NOAUTH" = "303" ]' "got $SETTINGS_POST_NOAUTH"

# ==========================================
# Registration code
# ==========================================
echo ""
echo "--- Registration Code ---"

# Submit empty registration code
REG_EMPTY=$(auth_post "$COOKIE_JAR" "$BASE/settings/registration" -d "registration_code=")
check "Empty registration code shows error" 'echo "$REG_EMPTY" | grep -qi "required\|error\|danger"' ""

# Submit a registration code (mock accepts any)
REG_OK=$(auth_post "$COOKIE_JAR" "$BASE/settings/registration" -d "registration_code=TEST123")
check "Registration code accepted" 'echo "$REG_OK" | grep -qi "success\|accepted"' ""

# Registration code result targets #reg-code-result
check "Registration form targets reg-code-result" 'echo "$SETTINGS_PAGE" | grep -q "hx-target=\"#reg-code-result\""' ""

# Registration without auth
REG_NOAUTH=$(curl -s -o /dev/null -w "%{http_code}" -X POST -d "registration_code=X" "$BASE/settings/registration" 2>/dev/null)
check "Registration without auth redirects (303)" '[ "$REG_NOAUTH" = "303" ]' "got $REG_NOAUTH"

# ==========================================
# Players DB page
# ==========================================
echo ""
echo "--- Players DB ---"

PDB_PAGE=$(auth_get "$COOKIE_JAR" "$BASE/players-db")
check "Players DB page renders" 'echo "$PDB_PAGE" | grep -qi "Players Database"' ""

# Mock returns empty state with Enabled=false (default zero value)
check "Players DB shows disabled message" 'echo "$PDB_PAGE" | grep -qi "disabled\|currently disabled\|feature.*off"' ""
check "Players DB shows link to Settings" 'echo "$PDB_PAGE" | grep -qi "href.*settings"' ""

# When enabled, should show download button (check template source)
check "Template has download-players-db link" 'grep -q "download-players-db" internal/server/pages/placeholders.templ' ""
check "Template shows player count when enabled" 'grep -q "PlayerCount\|Player Count" internal/server/pages/placeholders.templ' ""

# Auth required
PDB_NOAUTH=$(curl -s -o /dev/null -w "%{http_code}" "$BASE/players-db" 2>/dev/null)
check "Players DB without auth redirects (303)" '[ "$PDB_NOAUTH" = "303" ]' "got $PDB_NOAUTH"

# ==========================================
# Download Players DB
# ==========================================
echo ""
echo "--- Download Players DB ---"

DL_HEADERS=$(curl -s -D - -o /dev/null -b "$COOKIE_JAR" "$BASE/download-players-db" 2>/dev/null)
check "Download sets Content-Disposition header" 'echo "$DL_HEADERS" | grep -qi "Content-Disposition.*attachment.*players.db"' \
  "Headers: $(echo "$DL_HEADERS" | grep -i disposition || echo 'NONE')"
check "Download sets application/octet-stream content type" 'echo "$DL_HEADERS" | grep -qi "Content-Type.*application/octet-stream"' ""

# Download is a standard link, not htmx (check template)
check "Download uses standard <a> tag" 'grep -q "<a href.*download-players-db" internal/server/pages/placeholders.templ' ""

DL_NOAUTH=$(curl -s -o /dev/null -w "%{http_code}" "$BASE/download-players-db" 2>/dev/null)
check "Download without auth redirects (303)" '[ "$DL_NOAUTH" = "303" ]' "got $DL_NOAUTH"

# ==========================================
# Navbar links (all pages should have nav)
# ==========================================
echo ""
echo "--- Navigation ---"

DASHBOARD=$(auth_get "$COOKIE_JAR" "$BASE/")
check "Dashboard has nav link to Archive" 'echo "$DASHBOARD" | grep -qi "href.*archived"' ""
check "Dashboard has nav link to Settings" 'echo "$DASHBOARD" | grep -qi "href.*settings"' ""
check "Dashboard has nav link to Players DB" 'echo "$DASHBOARD" | grep -qi "href.*players-db"' ""
check "Archive page has navbar" 'echo "$ARCHIVE_PAGE" | grep -qi "navbar"' ""
check "Settings page has navbar" 'echo "$SETTINGS_PAGE" | grep -qi "navbar"' ""
check "Players DB page has navbar" 'echo "$PDB_PAGE" | grep -qi "navbar"' ""

# ==========================================
# Mutex violation check (CLAUDE.md: No mutexes)
# ==========================================
echo ""
echo "--- Concurrency Pattern ---"

MUTEX_FILES=$(grep -rl "sync.Mutex\|sync.RWMutex" internal/ --include="*.go" | grep -v "_test.go" || true)
if [ -n "$MUTEX_FILES" ]; then
  echo "  FAIL: Mutex usage found in production code (CLAUDE.md says actor pattern only):"
  echo "$MUTEX_FILES" | while read f; do echo "    - $f"; done
  FAIL_COUNT=$((FAIL_COUNT + 1))
else
  echo "  PASS: No mutexes in production code"
  PASS_COUNT=$((PASS_COUNT + 1))
fi

# Clean up
rm -f "$COOKIE_JAR" "$COOKIE_JAR2" "$COOKIE_JAR" 2>/dev/null || true

echo ""
echo "=== Results ==="
echo "PASS: $PASS_COUNT"
echo "FAIL: $FAIL_COUNT"
exit $FAIL_COUNT
