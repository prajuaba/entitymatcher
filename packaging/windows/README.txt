WHAT THIS IS
----------------
A prebuilt Entity Matcher runtime for Windows 11 x64. No Go, no Node.js, and no build
step are needed on the target machine -- server.exe is already compiled and
frontend\dist already contains the built single-page app. server.exe serves both the
JSON API and the UI on a single port (default 8085); there is no separate frontend
port.

REQUIREMENTS
----------------
Windows 10/11 x64. Nothing else is required for the default in-memory run. Docker
Desktop is ONLY needed if Postgres persistence is wanted.

QUICK START
----------------
Run: .\Start-EntityMatcher.ps1
Then open: http://localhost:8085
Note: if PowerShell refuses to run the script because it is unsigned, run
Set-ExecutionPolicy -Scope Process -ExecutionPolicy Bypass
first, and note this only changes execution policy for that one PowerShell session/
process, not permanently or system-wide.

DEMO ACCOUNTS
----------------
admin, engineer_alex, reviewer_sarah, auditor_mike -- all with password password123.
These must be changed before any real use.

STORAGE MODES
----------------
1) Default: in-memory store. Zero setup. All data is lost when the server stops.
2) Bundled PostgreSQL (no Docker install required): run
.\Start-EntityMatcher.ps1 -Postgres
Data persists in the pgsql-data folder inside the package. On first -Postgres run,
it initialises itself (takes a few seconds), generates a random superuser password
into pg-password.txt, and listens on 127.0.0.1 only (never exposed to the network).
Deleting the pgsql-data folder resets it and destroys all data. Note: PostgreSQL
will NOT run from an elevated/Administrator PowerShell window -- it must be started
from a normal PowerShell window.
3) External/Docker Postgres: persistent. Steps:
  docker compose -f docker-compose.postgres.yml up -d
  .\Start-EntityMatcher.ps1 -DatabaseUrl "postgres://postgres:postgrespassword@localhost:5432/entity_matcher?sslmode=disable"

ENVIRONMENT VARIABLES
----------------
PORT                 default 8085      HTTP listen port
JWT_SECRET           default random each start   Token signing key. If unset, tokens
                      do not survive a restart.
DATABASE_URL         default unset     PostgreSQL DSN. When unset the server uses an
                      in-memory store -- fully functional but everything is lost on
                      restart.
CONNECTOR_FILE_ROOT  set by launcher  Directory the connector endpoints may read
                      server-side file_path values from. The server itself denies
                      ALL of them when this is unset; Start-EntityMatcher.ps1
                      defaults it to this package's connector-data folder, so the
                      reads are confined to that folder. Set it yourself before
                      launching to point somewhere else.
CORS_ORIGINS         default unset     Comma-separated allowlist; same-origin only
                      when unset.
MAX_INGEST_RECORDS   default 500000    Rows per side per ingest.
GEMINI_API_KEY       default unset     Enables the LLM resolver; falls back to a
                      local rule-based analyzer when unset.

COMMON FLAGS
----------------
-Port        overrides the listen port
-DatabaseUrl sets DATABASE_URL for that run
-Foreground  runs the server attached to the current console instead of as a background
             process with logs redirected to files
-Postgres    starts and uses the bundled PostgreSQL database instead of the
             in-memory store (see STORAGE MODES); cannot be combined with
             -DatabaseUrl
-PgPort      overrides the port the bundled PostgreSQL listens on (default 5432)

LOGS
----------------
server.log and server.err.log are written to the package root (only in background
mode, since -Foreground attaches directly to the console). postgres.log is also
written to the package root when -Postgres is used; it is the bundled PostgreSQL
server's own log.

DO NOT REARRANGE THESE FOLDERS
----------------
server.exe resolves the UI at the relative path ..\frontend\dist based on its
own current working directory, so the backend\ and frontend\ folders must
remain siblings in the same parent folder in their current relative positions
-- moving or renaming either one breaks this, and the symptom is that the UI
returns 404 errors while the JSON API still answers normally. Note also that pgsql\
and pgsql-data\ (when present) must also stay in the package root alongside
backend\ and frontend\, for the same reason.

WINDOWS FIREWALL
----------------
The first run may prompt Windows to allow the listener through the firewall; the
server binds to localhost only by default, so this is expected and safe to allow.

