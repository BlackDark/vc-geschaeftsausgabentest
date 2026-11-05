---
status: accepted
date: 2026-10-19
---
# Serialize submission through one SQLite connection and one transaction

SQLite runs with a single connection, so submissions are serialized. One write transaction covers the final daily-allocation check, number allocation, artifact publication and the status change; files are staged and checksummed first, moved into storage before the commit and removed if it fails, and startup deletes exports no version references. This prevents double-spending a daily allowance and gapless numbers being consumed by failed attempts, at the cost of write throughput that a one-user tool does not need. Moving to another database or concurrent writers requires redesigning this guarantee.
