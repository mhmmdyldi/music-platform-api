# 0004. Environment-variable configuration with enforced environment isolation

**Status:** accepted

## Context

Stage and production must never touch each other, and secrets must never be
committed. An earlier project used as a reference had two configuration prefixes read by
different binaries (a documented production trap), and treated an empty CORS
list as "allow everything".

## Decision

- All configuration comes from environment variables, parsed and validated in
  one package. Every problem is reported at once at startup.
- Committed, non-secret env files per environment in `config/`. Tests load
  them and fail if one is invalid or contains a secret-looking key.
- Secrets come from AWS Secrets Manager, namespaced per environment.
- The database name must end with `_<APP_ENV>`, so a misrouted URL fails at
  startup instead of reading or writing another environment's data.
- Deployed environments additionally require TLS to the database, JSON logs
  and no debug logging.
- No configuration variable contains the product codename.

## Consequences

- Misconfiguration is caught on the first start, with a full list of problems.
- Database naming is a hard convention for every environment.
