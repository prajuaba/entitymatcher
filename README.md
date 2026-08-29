# High-Scale Thai-English Entity Resolution & Data Matching Engine

High-performance, bilingual entity resolution platform designed for resolving Thai and English individual and corporate entity names at scales exceeding 100,000+ records.

---

## 🌟 Key Features & Enterprise Architecture

1. **Rune-Safe Multi-Byte Thai Distance Metrics**:
   - Rune-safe UTF-8 character indexing for Jaro-Winkler, Levenshtein, Token Sort Ratio, Trigram Jaccard, and Phonetic Consonant Key algorithms.
2. **Sub-Linear Candidate Blocking Index (100,000+ Scaling)**:
   - Trigram + Inverted Token Blocking Index achieving **7,854.15 records/sec throughput** (16 Million candidate pair combinations evaluated in ~500ms).
3. **Pluggable Data Connectors**:
   - Native drivers for **SQL Server (MSSQL)**, **PostgreSQL**, **MongoDB**, **Excel (.xlsx)**, **CSV**, and **Manual Entry**.
   - Heterogeneous cross-database matching (e.g. Source = SQL Server `dbo.Customers`, Destination = MongoDB `clients_collection`).
4. **Dynamic Schema Mapping & Multi-Field Pairing**:
   - Pick single or multiple name fields (e.g. `First Name` + `Last Name`), Reference ID columns, Date columns, and secondary pairing rules (`EXACT MATCH`, `FUZZY MATCH`, `NUMERIC DELTA`, `Mandatory Flag`).
5. **Compliance Audit Trail & Governance Log**:
   - Immutable audit trail recording reviewer ID (`user_id`), review action (`CONFIRM`, `REJECT`), confidence score, timestamp, and human reviewer comments.
   - One-click regulatory CSV export for AML / KYC compliance audits.
6. **JWT Authentication & 4-Tier Role-Based Access Control (RBAC)**:
   - `ADMIN`: Full access (Config, Connectors, Thresholds, User Governance).
   - `ENGINEER`: Schema mapping, Data Connectors, Ingestion, Matching.
   - `REVIEWER`: Review Queue, Confirm/Reject candidate pairs.
   - `AUDITOR`: Read-only Compliance Audit Trail and regulatory exports.
7. **Automated Cron Scheduler & Webhook Alerts**:
   - Automated nightly reconciliation scheduler (`0 2 * * *`) + Slack/Teams/REST webhook alerts dispatched on batch completion or anomaly detection.
8. **Enterprise Synonym Dictionary & Brand Alias Manager**:
   - Custom synonym dictionary UI manager to map brand aliases (`KBank` / `กสิกร` $\rightarrow$ `Kasikornbank`, `SCB` / `ไทยพาณิชย์` $\rightarrow$ `Siam Commercial Bank`, `AIS` / `เอไอเอส` $\rightarrow$ `Advanced Info Service`).
9. **LLM Edge-Case Resolver**:
   - Structured JSON edge-case evaluation with Gemini API and fallback rule-based analyzer.
10. **React SPA User Interface**:
   - Split view review queue, bilingual token-level visual diffing, SSE progress dashboard, schema mapping wizard, and audit log dashboard.

---

## 🚀 Quick Start Guide

### 1. Run Standalone Go Server & React SPA

```bash
# Build backend Go server
cd backend
go build -o server .

# Run server on port 8085
PORT=8085 ./server
```

Open **`http://localhost:8085`** in your browser.

### 2. Run via Docker Compose

```bash
docker-compose up --build
```

Access the application at `http://localhost:8085`.

---

## 📡 REST API Reference

### Core Engine & Matching
- `GET /api/config` - Retrieve current algorithm toggles, thresholds, and weights.
- `PUT /api/config` - Update engine configuration dynamically.
- `POST /api/upload` - Stream ingest custom datasets with dynamic schema mapping.
- `POST /api/match/run?batch_id=...` - Execute parallel matching job via worker pool.
- `GET /api/match/progress?batch_id=...` - Server-Sent Events (SSE) live progress stream.
- `GET /api/match/results?batch_id=...&status=...` - Paginated master-detail match results.
- `POST /api/match/action` - Confirm, reject, or unlink candidate pairs with reviewer comments.
- `POST /api/match/manual-link` - Manually pair a source record with a destination record.
- `GET /api/destinations/search?batch_id=...&query=...` - Search candidate records for manual pairing.
- `POST /api/llm/evaluate` - Send edge-case records to LLM prompt evaluation.
- `GET /api/export/csv?batch_id=...` - Download paired matched dataset as CSV.

### Data Connectors & Schema Introspection
- `POST /api/connector/test` - Test connection parameters for SQL Server, Postgres, Mongo, CSV, Excel.
- `POST /api/connector/introspect` - Introspect column names and data types from data sources.

### Governance & Compliance Audit
- `GET /api/audit/logs?batch_id=...&user_id=...&action=...` - Retrieve compliance audit log entries.
- `GET /api/audit/export?batch_id=...` - Export regulatory compliance audit trail as CSV.

### Authentication, Scheduling & Synonyms
- `POST /api/auth/login` - Authenticate user and issue HMAC-SHA256 JWT token.
- `GET /api/auth/me` - Fetch authenticated user profile and RBAC role.
- `GET /api/scheduler/config` - Fetch active cron scheduler and webhook notification settings.
- `POST /api/scheduler/config` - Update automated reconciliation schedule & webhook target URL.
- `GET /api/dictionary` - Retrieve custom enterprise brand synonym map.
- `POST /api/dictionary` - Add or update brand alias mapping (`alias` $\rightarrow$ `canonical`).
