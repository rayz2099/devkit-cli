# Write Access forwards

Access `read` is the Gate allowlist. Access `write` forwards Query to the Driver with no in-process restriction.

A second write allowlist was rejected; it would rot like sql-manual pages. Hard-rejecting `EVAL` on write profiles was rejected: write means unrestricted. The server account is the remaining control.
