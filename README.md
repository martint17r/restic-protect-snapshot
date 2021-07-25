# restic protect snapshot

This is a proof of concept on how to provide immutable backups with restic on s3 compatible storage systems using object locks.

This must not be used on production systems.

## Prerequisites

* restic compiled from https://github.com/martint17r/restic/tree/restore-dryrun
* set RPS_RESTIC_COMMAND to the binary build from the above branch
* set RESTIC_REPOSITORY to an s3 location containing a restic repository
* set RESTIC_PASSWORD_COMMAND or RESTIC_PASSWORD_FILE

## Run

```bash

RPS_RESTIC_COMMAND=../restic/restic go run .
```

