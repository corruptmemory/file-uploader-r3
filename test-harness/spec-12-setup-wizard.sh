#!/usr/bin/env bash
# Spec 12: Setup Wizard — Integration Tests
# Tests the setup wizard HTTP endpoints, navigation, validation, and state transitions.
set -euo pipefail

PASS_COUNT=0
FAIL_COUNT=0
PORT=9877
BASE="http://localhost:$PORT"
APP_PID=""
TMPDIR_SETUP=""

pass() { PASS_COUNT=$((PASS_COUNT + 1)); echo "  PASS: $1"; }
fail() { FAIL_COUNT=$((FAIL_COUNT + 1)); echo "  FAIL: $1"; }

cleanup() {
    if [[ -n "$APP_PID" ]]; then
        kill "$APP_PID" 2>/dev/null || true
        wait "$APP_PID" 2>/dev/null || true
    fi
    if [[ -n "$TMPDIR_SETUP" ]]; then
        rm -rf "$TMPDIR_SETUP"
    fi
}
trap cleanup EXIT

# Start app in SETUP state (no --mock, empty config → NeedsSetup() returns true)
start_app_setup() {
    TMPDIR_SETUP=$(mktemp -d)
    local EMPTY_CONF="$TMPDIR_SETUP/empty.toml"
    local KEY_FILE="$TMPDIR_SETUP/key.txt"
    touch "$EMPTY_CONF"
    echo "testsigningkeymaterial" > "$KEY_FILE"
    chmod 0600 "$KEY_FILE"
    ./file-uploader -c "$EMPTY_CONF" --signing-key-file "$KEY_FILE" -p "$PORT" &>/dev/null &
    APP_PID=$!
    for i in $(seq 1 30); do
        if curl -s "$BASE/health" | grep -q "ok"; then
            return 0
        fi
        sleep 0.2
    done
    echo "FATAL: app did not start"
    exit 1
}

# Start app in RUNNING state (--mock)
start_app_running() {
    ./file-uploader --mock -p "$PORT" &>/dev/null &
    APP_PID=$!
    for i in $(seq 1 30); do
        if curl -s "$BASE/health" | grep -q "ok"; then
            return 0
        fi
        sleep 0.2
    done
    echo "FATAL: app did not start"
    exit 1
}

stop_app() {
    if [[ -n "$APP_PID" ]]; then
        kill "$APP_PID" 2>/dev/null || true
        wait "$APP_PID" 2>/dev/null || true
        APP_PID=""
    fi
    if [[ -n "$TMPDIR_SETUP" ]]; then
        rm -rf "$TMPDIR_SETUP"
        TMPDIR_SETUP=""
    fi
}

echo "=== Spec 12: Setup Wizard Tests ==="
echo

# ------------------------------------------------------------------
# Test group: Initial state and redirection
# ------------------------------------------------------------------
echo "--- State Redirection (spec section 6) ---"
start_app_setup

# When app starts with no config, it should be in setup state
# Non-setup routes should redirect to /setup
CODE=$(curl -s -o /dev/null -w "%{http_code}" --max-redirs 0 "$BASE/")
if [[ "$CODE" == "303" ]]; then pass "GET / → 303 redirect (setup state)"
else fail "GET / → $CODE (expected 303)"; fi

LOC=$(curl -s -D - -o /dev/null --max-redirs 0 "$BASE/" 2>/dev/null | grep -i "^location:" | tr -d '\r' | awk '{print $2}')
if echo "$LOC" | grep -q "/setup"; then pass "GET / redirect target is /setup"
else fail "GET / redirect target: $LOC (expected /setup)"; fi

CODE=$(curl -s -o /dev/null -w "%{http_code}" --max-redirs 0 "$BASE/login")
if [[ "$CODE" == "303" ]]; then pass "GET /login → 303 redirect (setup state)"
else fail "GET /login → $CODE (expected 303)"; fi

CODE=$(curl -s -o /dev/null -w "%{http_code}" --max-redirs 0 "$BASE/settings")
if [[ "$CODE" == "303" ]]; then pass "GET /settings → 303 redirect (setup state)"
else fail "GET /settings → $CODE (expected 303)"; fi

# Health should work regardless of state
CODE=$(curl -s -o /dev/null -w "%{http_code}" "$BASE/health")
if [[ "$CODE" == "200" ]]; then pass "GET /health → 200 (available in any state)"
else fail "GET /health → $CODE (expected 200)"; fi

# Static assets should work
CODE=$(curl -s -o /dev/null -w "%{http_code}" "$BASE/vendor/htmx.min.js")
if [[ "$CODE" == "200" ]]; then pass "GET /vendor/htmx.min.js → 200 (static in any state)"
else fail "GET /vendor/htmx.min.js → $CODE (expected 200)"; fi

stop_app
echo

# ------------------------------------------------------------------
# Test group: GET /setup renders wizard
# ------------------------------------------------------------------
echo "--- GET /setup renders wizard (spec section 4) ---"
start_app_setup

BODY=$(curl -s "$BASE/setup")
if echo "$BODY" | grep -q "Setup Wizard"; then pass "setup page has 'Setup Wizard' title"
else fail "setup page missing 'Setup Wizard' title"; fi

if echo "$BODY" | grep -q "Welcome"; then pass "setup page starts on Welcome step"
else fail "setup page missing Welcome step"; fi

if echo "$BODY" | grep -q "Get Started"; then pass "Welcome has 'Get Started' button"
else fail "Welcome missing 'Get Started' button"; fi

# Back button should NOT be on Welcome step (spec section 3)
if echo "$BODY" | grep -q 'setup/back'; then fail "Welcome step has Back button (spec says hidden)"
else pass "Welcome step has no Back button"; fi

# Check for htmx integration
if echo "$BODY" | grep -q "hx-post"; then pass "setup uses hx-post for navigation"
else fail "setup missing hx-post"; fi

if echo "$BODY" | grep -q "hx-target"; then pass "setup uses hx-target"
else fail "setup missing hx-target"; fi

if echo "$BODY" | grep -q '#wizard-content\|wizard-content'; then pass "setup targets wizard-content"
else fail "setup missing wizard-content target"; fi

# current_step hidden field
if echo "$BODY" | grep -q 'name="current_step"'; then pass "Welcome has current_step hidden field"
else fail "Welcome missing current_step field"; fi

if echo "$BODY" | grep -q 'value="0"'; then pass "Welcome current_step value is 0"
else fail "Welcome current_step value not 0"; fi

# CSS and JS references
if echo "$BODY" | grep -q "tokens.css"; then pass "Setup page references tokens.css"
else fail "Setup page missing tokens.css reference"; fi

if echo "$BODY" | grep -q "app.css"; then pass "Setup page references app.css"
else fail "Setup page missing app.css reference"; fi

if echo "$BODY" | grep -q "htmx"; then pass "Setup page references htmx"
else fail "Setup page missing htmx reference"; fi

stop_app
echo

# ------------------------------------------------------------------
# Test group: Forward navigation through all steps
# ------------------------------------------------------------------
echo "--- Forward Navigation (spec section 3) ---"
start_app_setup

# Step 0 → 1 (Welcome → Endpoint)
STEP1=$(curl -s -X POST "$BASE/setup/next" \
    -H "HX-Request: true" \
    -d "current_step=0")
if echo "$STEP1" | grep -q "Service Endpoint\|Endpoint"; then pass "Welcome → Endpoint step"
else fail "Welcome → Endpoint: got $(echo "$STEP1" | head -1)"; fi

if echo "$STEP1" | grep -q 'name="environment"'; then pass "Endpoint has environment input"
else fail "Endpoint missing environment input"; fi

if echo "$STEP1" | grep -q 'name="endpoint"'; then pass "Endpoint has endpoint input"
else fail "Endpoint missing endpoint input"; fi

# Endpoint step should have Back button
if echo "$STEP1" | grep -q 'setup/back'; then pass "Endpoint step has Back button"
else fail "Endpoint step missing Back button"; fi

# Step 1 → 2 (Endpoint → ServiceCredentials)
STEP2=$(curl -s -X POST "$BASE/setup/next" \
    -H "HX-Request: true" \
    -d "current_step=1&endpoint=https://api.example.com&environment=testing")
if echo "$STEP2" | grep -q "Service Credentials\|Credentials\|Registration"; then pass "Endpoint → Credentials step"
else fail "Endpoint → Credentials: got $(echo "$STEP2" | head -1)"; fi

if echo "$STEP2" | grep -q 'name="registration_code"'; then pass "Credentials has registration_code input"
else fail "Credentials missing registration_code input"; fi

# Step 2 → 3 (Credentials → PlayerIDHasher)
STEP3=$(curl -s -X POST "$BASE/setup/next" \
    -H "HX-Request: true" \
    -d "current_step=2&registration_code=test-code")
if echo "$STEP3" | grep -q "Player ID Hasher\|Hasher\|Pepper"; then pass "Credentials → Hasher step"
else fail "Credentials → Hasher: got $(echo "$STEP3" | head -1)"; fi

if echo "$STEP3" | grep -q 'name="pepper"'; then pass "Hasher has pepper input"
else fail "Hasher missing pepper input"; fi

if echo "$STEP3" | grep -q "argon2"; then pass "Hasher shows argon2 algorithm"
else fail "Hasher missing argon2"; fi

if echo "$STEP3" | grep -q "readonly"; then pass "Hash algorithm is read-only"
else fail "Hash algorithm not read-only"; fi

# Step 3 → 4 (Hasher → UsePlayersDB)
STEP4=$(curl -s -X POST "$BASE/setup/next" \
    -H "HX-Request: true" \
    -d "current_step=3&pepper=testpepper123&hash_algorithm=argon2")
if echo "$STEP4" | grep -q "Players Database\|UsePlayersDB\|use_players_db"; then pass "Hasher → UsePlayersDB step"
else fail "Hasher → UsePlayersDB: got $(echo "$STEP4" | head -1)"; fi

if echo "$STEP4" | grep -q 'name="use_players_db"'; then pass "UsePlayersDB has radio buttons"
else fail "UsePlayersDB missing radio buttons"; fi

if echo "$STEP4" | grep -q 'value="true"'; then pass "UsePlayersDB has Yes option"
else fail "UsePlayersDB missing Yes option"; fi

if echo "$STEP4" | grep -q 'value="false"'; then pass "UsePlayersDB has No option"
else fail "UsePlayersDB missing No option"; fi

# Step 4 → 5 (UsePlayersDB → Done)
STEP5=$(curl -s -X POST "$BASE/setup/next" \
    -H "HX-Request: true" \
    -d "current_step=4&use_players_db=true")
if echo "$STEP5" | grep -q "Setup Complete\|Complete\|Done"; then pass "UsePlayersDB → Done step"
else fail "UsePlayersDB → Done: got $(echo "$STEP5" | head -1)"; fi

if echo "$STEP5" | grep -q "login"; then pass "Done step has link to login"
else fail "Done step missing login link"; fi

stop_app
echo

# ------------------------------------------------------------------
# Test group: Backward navigation preserves values
# ------------------------------------------------------------------
echo "--- Backward Navigation + Value Preservation (spec section 3) ---"
start_app_setup

# Navigate forward to endpoint, set values, go forward, then back
curl -s -X POST "$BASE/setup/next" -H "HX-Request: true" -d "current_step=0" > /dev/null
curl -s -X POST "$BASE/setup/next" -H "HX-Request: true" \
    -d "current_step=1&endpoint=https://preserved.example.com&environment=myenv" > /dev/null

# Now go back from credentials to endpoint
BACK_RESULT=$(curl -s -X POST "$BASE/setup/back" \
    -H "HX-Request: true" \
    -d "current_step=2")
if echo "$BACK_RESULT" | grep -q "Service Endpoint\|Endpoint"; then pass "Back from Credentials → Endpoint"
else fail "Back from Credentials didn't reach Endpoint: $(echo "$BACK_RESULT" | head -1)"; fi

if echo "$BACK_RESULT" | grep -q "preserved.example.com"; then pass "Endpoint URL preserved after back"
else fail "Endpoint URL not preserved"; fi

if echo "$BACK_RESULT" | grep -q "myenv"; then pass "Environment preserved after back"
else fail "Environment not preserved"; fi

# Go back to Welcome
BACK_WELCOME=$(curl -s -X POST "$BASE/setup/back" \
    -H "HX-Request: true" \
    -d "current_step=1")
if echo "$BACK_WELCOME" | grep -q "Welcome"; then pass "Back from Endpoint → Welcome"
else fail "Back from Endpoint didn't reach Welcome: $(echo "$BACK_WELCOME" | head -1)"; fi

# Navigate forward again past credentials to hasher, then back
curl -s -X POST "$BASE/setup/next" -H "HX-Request: true" -d "current_step=0" > /dev/null
curl -s -X POST "$BASE/setup/next" -H "HX-Request: true" \
    -d "current_step=1&endpoint=https://preserved.example.com&environment=myenv" > /dev/null
curl -s -X POST "$BASE/setup/next" -H "HX-Request: true" \
    -d "current_step=2&registration_code=testcode" > /dev/null
curl -s -X POST "$BASE/setup/next" -H "HX-Request: true" \
    -d "current_step=3&pepper=savedpepper&hash_algorithm=argon2" > /dev/null

# Back from UsePlayersDB → Hasher should preserve pepper
BACK_HASHER=$(curl -s -X POST "$BASE/setup/back" \
    -H "HX-Request: true" \
    -d "current_step=4")
if echo "$BACK_HASHER" | grep -q "savedpepper"; then pass "Pepper preserved after back from UsePlayersDB"
else fail "Pepper not preserved after back"; fi

stop_app
echo

# ------------------------------------------------------------------
# Test group: Validation failures re-render with errors
# ------------------------------------------------------------------
echo "--- Validation Failures (spec section 2) ---"
start_app_setup

# Navigate to endpoint step
curl -s -X POST "$BASE/setup/next" -H "HX-Request: true" -d "current_step=0" > /dev/null

# Empty endpoint
EMPTY_EP=$(curl -s -X POST "$BASE/setup/next" \
    -H "HX-Request: true" \
    -d "current_step=1&endpoint=&environment=testing")
if echo "$EMPTY_EP" | grep -qi "required\|error"; then pass "Empty endpoint → validation error"
else fail "Empty endpoint accepted (should fail)"; fi

# Empty environment
EMPTY_ENV=$(curl -s -X POST "$BASE/setup/next" \
    -H "HX-Request: true" \
    -d "current_step=1&endpoint=https://api.example.com&environment=")
if echo "$EMPTY_ENV" | grep -qi "required\|error"; then pass "Empty environment → validation error"
else fail "Empty environment accepted"; fi

# Environment too short (< 3 chars)
SHORT_ENV=$(curl -s -X POST "$BASE/setup/next" \
    -H "HX-Request: true" \
    -d "current_step=1&endpoint=https://api.example.com&environment=ab")
if echo "$SHORT_ENV" | grep -qi "3-15\|character\|error"; then pass "Short environment (2 chars) → validation error"
else fail "Short environment accepted"; fi

# Environment too long (> 15 chars)
LONG_ENV=$(curl -s -X POST "$BASE/setup/next" \
    -H "HX-Request: true" \
    -d "current_step=1&endpoint=https://api.example.com&environment=1234567890123456")
if echo "$LONG_ENV" | grep -qi "3-15\|character\|error"; then pass "Long environment (16 chars) → validation error"
else fail "Long environment accepted"; fi

# Navigate to hasher step
curl -s -X POST "$BASE/setup/next" -H "HX-Request: true" \
    -d "current_step=1&endpoint=https://api.example.com&environment=testing" > /dev/null
curl -s -X POST "$BASE/setup/next" -H "HX-Request: true" \
    -d "current_step=2&registration_code=test" > /dev/null

# Pepper too short (< 5 chars)
SHORT_PEPPER=$(curl -s -X POST "$BASE/setup/next" \
    -H "HX-Request: true" \
    -d "current_step=3&pepper=abc&hash_algorithm=argon2")
if echo "$SHORT_PEPPER" | grep -qi "5 char\|pepper\|at least\|error"; then pass "Short pepper (3 chars) → validation error"
else fail "Short pepper accepted"; fi

# Empty pepper
EMPTY_PEPPER=$(curl -s -X POST "$BASE/setup/next" \
    -H "HX-Request: true" \
    -d "current_step=3&pepper=&hash_algorithm=argon2")
if echo "$EMPTY_PEPPER" | grep -qi "required\|error"; then pass "Empty pepper → validation error"
else fail "Empty pepper accepted"; fi

# Empty registration code
stop_app
start_app_setup
curl -s -X POST "$BASE/setup/next" -H "HX-Request: true" -d "current_step=0" > /dev/null
curl -s -X POST "$BASE/setup/next" -H "HX-Request: true" \
    -d "current_step=1&endpoint=https://api.example.com&environment=testing" > /dev/null
EMPTY_CODE=$(curl -s -X POST "$BASE/setup/next" \
    -H "HX-Request: true" \
    -d "current_step=2&registration_code=")
if echo "$EMPTY_CODE" | grep -qi "required\|error\|code"; then pass "Empty registration code → validation error"
else fail "Empty registration code accepted"; fi

stop_app
echo

# ------------------------------------------------------------------
# Test group: Step skipping prevention
# ------------------------------------------------------------------
echo "--- Step Skipping Prevention (spec section 3) ---"
start_app_setup

# App starts at step 0 (Welcome). Try to submit step 3 data.
SKIP_BODY=$(curl -s -X POST "$BASE/setup/next" \
    -H "HX-Request: true" \
    -d "current_step=3&pepper=testpepper&hash_algorithm=argon2")
# Should NOT advance to UsePlayersDB — should return current step (Welcome)
if echo "$SKIP_BODY" | grep -q "Welcome\|Get Started"; then pass "Cannot skip from Welcome to step 3 result"
elif echo "$SKIP_BODY" | grep -q "Players Database"; then fail "Step skip from 0→3 succeeded!"
else pass "Step skip rejected (returned current state)"; fi

# Try to go directly to UsePlayersDB → Done from Welcome
SKIP_BODY2=$(curl -s -X POST "$BASE/setup/next" \
    -H "HX-Request: true" \
    -d "current_step=4&use_players_db=true")
if echo "$SKIP_BODY2" | grep -q "Setup Complete\|Done"; then fail "Skipped to Done from Welcome!"
else pass "Cannot skip to Done from Welcome"; fi

stop_app
echo

# ------------------------------------------------------------------
# Test group: CSRF protection
# ------------------------------------------------------------------
echo "--- CSRF Protection (security) ---"
start_app_setup

# POST without HX-Request header should be rejected
CSRF_CODE=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$BASE/setup/next" \
    -d "current_step=0")
if [[ "$CSRF_CODE" == "403" ]]; then pass "POST /setup/next without HX-Request → 403"
else fail "POST /setup/next without HX-Request → $CSRF_CODE (expected 403)"; fi

CSRF_BACK=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$BASE/setup/back" \
    -d "current_step=1")
if [[ "$CSRF_BACK" == "403" ]]; then pass "POST /setup/back without HX-Request → 403"
else fail "POST /setup/back without HX-Request → $CSRF_BACK (expected 403)"; fi

stop_app
echo

# ------------------------------------------------------------------
# Test group: Security headers
# ------------------------------------------------------------------
echo "--- Security Headers ---"
start_app_setup

HEADERS=$(curl -s -D - -o /dev/null "$BASE/setup")
if echo "$HEADERS" | grep -qi "X-Frame-Options: DENY"; then pass "X-Frame-Options: DENY present"
else fail "X-Frame-Options missing or wrong"; fi

if echo "$HEADERS" | grep -qi "X-Content-Type-Options: nosniff"; then pass "X-Content-Type-Options: nosniff present"
else fail "X-Content-Type-Options missing"; fi

if echo "$HEADERS" | grep -qi "Content-Security-Policy"; then pass "Content-Security-Policy present"
else fail "Content-Security-Policy missing"; fi

stop_app
echo

# ------------------------------------------------------------------
# Test group: State transition on Done
# ------------------------------------------------------------------
echo "--- State Transition on Done (spec section 2 & acceptance criteria) ---"
start_app_setup

# Complete the full wizard
curl -s -X POST "$BASE/setup/next" -H "HX-Request: true" -d "current_step=0" > /dev/null
curl -s -X POST "$BASE/setup/next" -H "HX-Request: true" \
    -d "current_step=1&endpoint=https://api.example.com&environment=testing" > /dev/null
curl -s -X POST "$BASE/setup/next" -H "HX-Request: true" \
    -d "current_step=2&registration_code=test" > /dev/null
curl -s -X POST "$BASE/setup/next" -H "HX-Request: true" \
    -d "current_step=3&pepper=testpepper123&hash_algorithm=argon2" > /dev/null
DONE_RESP=$(curl -s -X POST "$BASE/setup/next" -H "HX-Request: true" \
    -d "current_step=4&use_players_db=true")
if echo "$DONE_RESP" | grep -q "Setup Complete\|Complete\|Done"; then pass "Wizard completes with Done message"
else fail "Wizard completion: $(echo "$DONE_RESP" | head -1)"; fi

# Wait a moment for async state transition
sleep 2

# After Done, app should be in RunningApp state — /setup should redirect to /
SETUP_AFTER=$(curl -s -o /dev/null -w "%{http_code}" --max-redirs 0 "$BASE/setup")
if [[ "$SETUP_AFTER" == "303" ]]; then pass "After Done: GET /setup → 303 (now in RunningApp)"
else fail "After Done: GET /setup → $SETUP_AFTER (expected 303 redirect)"; fi

# Login page should be accessible (RunningApp state)
LOGIN_CODE=$(curl -s -o /dev/null -w "%{http_code}" "$BASE/login")
if [[ "$LOGIN_CODE" == "200" ]]; then pass "After Done: GET /login → 200"
else fail "After Done: GET /login → $LOGIN_CODE"; fi

# Config file should have been written
if [[ -f "$TMPDIR_SETUP/empty.toml" ]]; then
    CONF_CONTENT=$(cat "$TMPDIR_SETUP/empty.toml")
    if echo "$CONF_CONTENT" | grep -q "endpoint"; then pass "Config file written with endpoint"
    else fail "Config file missing endpoint after wizard"; fi
    if echo "$CONF_CONTENT" | grep -q "testing"; then pass "Config file has environment value"
    else fail "Config file missing environment"; fi
    if echo "$CONF_CONTENT" | grep -q "testpepper123"; then pass "Config file has pepper value"
    else fail "Config file missing pepper"; fi
else
    fail "Config file does not exist after wizard completion"
fi

stop_app
echo

# ------------------------------------------------------------------
# Test group: POST method verification
# ------------------------------------------------------------------
echo "--- HTTP Method Verification ---"
start_app_setup

# GET /setup/next should not be allowed (only POST registered under POST route)
GET_NEXT=$(curl -s -o /dev/null -w "%{http_code}" "$BASE/setup/next")
if [[ "$GET_NEXT" == "405" ]]; then pass "GET /setup/next → 405 (POST only)"
else fail "GET /setup/next → $GET_NEXT (expected 405)"; fi

# GET /setup/back should not be allowed
GET_BACK=$(curl -s -o /dev/null -w "%{http_code}" "$BASE/setup/back")
if [[ "$GET_BACK" == "405" ]]; then pass "GET /setup/back → 405 (POST only)"
else fail "GET /setup/back → $GET_BACK (expected 405)"; fi

# Unknown action
UNK_ACTION=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$BASE/setup/foobar" \
    -H "HX-Request: true" -d "current_step=0")
if [[ "$UNK_ACTION" == "400" ]]; then pass "POST /setup/foobar → 400 (unknown action)"
else fail "POST /setup/foobar → $UNK_ACTION (expected 400)"; fi

stop_app
echo

# ------------------------------------------------------------------
# Test group: Rate limiting
# ------------------------------------------------------------------
echo "--- Rate Limiting ---"
start_app_setup

# Send 15 rapid requests — should eventually get 429
GOT_429=false
for i in $(seq 1 15); do
    RC=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$BASE/setup/next" \
        -H "HX-Request: true" -d "current_step=0")
    if [[ "$RC" == "429" ]]; then
        GOT_429=true
        break
    fi
done
if $GOT_429; then pass "Rate limiter kicks in after rapid requests (limit: 10/min)"
else fail "Rate limiter did not trigger after 15 rapid requests (expected 429)"; fi

stop_app
echo

# ------------------------------------------------------------------
# Test group: Error step (spec section 2, step 6)
# ------------------------------------------------------------------
echo "--- Error Step (spec section 2) ---"

# Template-level checks
if grep -q "stepError" internal/server/pages/setupsteps.templ; then pass "Error step template exists"
else fail "Error step template missing"; fi

if grep -q "Start Over" internal/server/pages/setupsteps.templ; then pass "Error step has 'Start Over' button"
else fail "Error step missing retry/back button"; fi

if grep -q "setup-error" internal/server/pages/setupsteps.templ; then pass "Error step uses setup-error class"
else fail "Error step missing setup-error class"; fi

echo

# ------------------------------------------------------------------
# Test group: CSS and design system
# ------------------------------------------------------------------
echo "--- CSS and Design System ---"
start_app_setup

CSS_CODE=$(curl -s -o /dev/null -w "%{http_code}" "$BASE/css/app.css")
if [[ "$CSS_CODE" == "200" ]]; then pass "app.css served"
else fail "app.css → $CSS_CODE"; fi

CSS_BODY=$(curl -s "$BASE/css/app.css")
if echo "$CSS_BODY" | grep -q "setup-wizard"; then pass "CSS has setup-wizard styles"
else fail "CSS missing setup-wizard"; fi

if echo "$CSS_BODY" | grep -q "setup-step"; then pass "CSS has setup-step styles"
else fail "CSS missing setup-step"; fi

if echo "$CSS_BODY" | grep -q "setup-error"; then pass "CSS has setup-error styles"
else fail "CSS missing setup-error"; fi

if echo "$CSS_BODY" | grep -q "btn-primary"; then pass "CSS has btn-primary styles"
else fail "CSS missing btn-primary"; fi

if echo "$CSS_BODY" | grep -q "form-group"; then pass "CSS has form-group styles"
else fail "CSS missing form-group"; fi

TOKENS_CODE=$(curl -s -o /dev/null -w "%{http_code}" "$BASE/css/tokens.css")
if [[ "$TOKENS_CODE" == "200" ]]; then pass "tokens.css served"
else fail "tokens.css → $TOKENS_CODE"; fi

stop_app
echo

# ------------------------------------------------------------------
# Test group: Content-Type headers
# ------------------------------------------------------------------
echo "--- Content-Type Headers ---"
start_app_setup

SETUP_CT=$(curl -s -D - -o /dev/null "$BASE/setup" | grep -i "^content-type:" | tr -d '\r')
if echo "$SETUP_CT" | grep -q "text/html"; then pass "GET /setup Content-Type: text/html"
else fail "GET /setup Content-Type: $SETUP_CT"; fi

# POST responses should also be text/html
NEXT_CT=$(curl -s -D - -o /dev/null -X POST "$BASE/setup/next" \
    -H "HX-Request: true" -d "current_step=0" | grep -i "^content-type:" | tr -d '\r')
if echo "$NEXT_CT" | grep -q "text/html"; then pass "POST /setup/next Content-Type: text/html"
else fail "POST /setup/next Content-Type: $NEXT_CT"; fi

stop_app
echo

# ------------------------------------------------------------------
# Test group: Dispatch map pattern (spec section 1)
# ------------------------------------------------------------------
echo "--- Dispatch Map Pattern (spec section 1) ---"

if grep -q "stepTransition" internal/setup/setup.go; then pass "stepTransition struct defined"
else fail "stepTransition struct missing"; fi

if grep -q "forwardHandlers" internal/setup/setup.go; then pass "forwardHandlers dispatch map used"
else fail "forwardHandlers dispatch map missing"; fi

if grep -q "backwardHandlers" internal/setup/setup.go; then pass "backwardHandlers dispatch map used"
else fail "backwardHandlers dispatch map missing"; fi

echo

# ------------------------------------------------------------------
# Test group: Running state — setup not accessible
# ------------------------------------------------------------------
echo "--- Running State: Setup Not Accessible ---"
start_app_running

CODE=$(curl -s -o /dev/null -w "%{http_code}" --max-redirs 0 "$BASE/setup")
if [[ "$CODE" == "303" ]]; then pass "GET /setup in RunningApp → 303 redirect"
else fail "GET /setup in RunningApp → $CODE (expected 303)"; fi

LOC=$(curl -s -D - -o /dev/null --max-redirs 0 "$BASE/setup" 2>/dev/null | grep -i "^location:" | tr -d '\r' | awk '{print $2}')
if echo "$LOC" | grep -q "/\$\|^/$"; then pass "Setup redirect target is / in RunningApp"
else fail "Setup redirect target: $LOC (expected /)"; fi

stop_app
echo

# ------------------------------------------------------------------
# Summary
# ------------------------------------------------------------------
echo "=== Results ==="
echo "PASS: $PASS_COUNT"
echo "FAIL: $FAIL_COUNT"

if [[ $FAIL_COUNT -gt 0 ]]; then
    exit 1
fi
