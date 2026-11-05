---
status: accepted
date: 2026-10-19
---
# Multi-user-capable data model with a single-user UI

The tool serves one employed managing director, but every record is owned by a user with server-side ownership checks, and the schema keeps an employer table although the UI shows one employer profile. Retrofitting ownership isolation or a second employer into claims, snapshots and numbering later would need data migration, while the UI can stay simple. Admins manage accounts and rate tables but cannot read other users' records.
