# Connector sample data

Mounted read-only into the backend container at `/data/connector`, which is what
`CONNECTOR_FILE_ROOT` points at in `docker-compose.yml`.

That variable gates every server-side `file_path` the connector endpoints will
open (backlog M1). **Unset denies all of them**, which is the safe default — but
it also means Test Connection, Introspect Schema and "Load Data & Start Batch"
have no reachable success path, which is why backlog T1 could not exercise them.
Pointing it at this directory makes those paths usable while keeping the blast
radius to this directory and nothing else.

The two CSVs here are deliberately tiny and cover the cases the matcher exists
for: a Thai company name against its abbreviated form, an English bank against
its `PLC` variant, a transposed personal name, and a Thai personal name with a
different honorific.

Replace them with your own data, or point `CONNECTOR_FILE_ROOT` somewhere else.
Nothing in the application depends on these files.
