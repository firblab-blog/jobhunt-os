# Backup and Restore

JobHunt OS stores its durable application data in the configured data directory.
For the recommended Docker Compose install, that directory is `./data` next to
`docker-compose.yml`.

## Back Up a Docker Compose Install

From the directory that contains `docker-compose.yml`:

```sh
docker compose down
tar -czf jobhunt-os-backup-$(date +%Y%m%d).tgz data
docker compose up -d
```

Stopping the container before archiving keeps the SQLite database and local
files in a quiet state while the backup is created.

Store the resulting `.tgz` somewhere outside the install directory, ideally on a
different disk or backup service.

## Restore or Move an Install

To restore or move JobHunt OS to another machine:

1. Install Docker and Docker Compose on the target machine.
2. Copy `docker-compose.yml` to the target install directory.
3. Copy or extract the backed-up `data/` directory into the same directory.
4. Start the app:

```sh
docker compose up -d
```

Then visit:

```text
http://127.0.0.1:8080
```

If you are restoring from the tar archive created above, the install directory
should end up with this shape:

```text
jobhunt-os/
  docker-compose.yml
  data/
    jobhunt-os.db
    documents/
    tmp/
```

If the target machine uses a different Linux UID/GID, set `JOBHUNT_UID` and
`JOBHUNT_GID` in `.env` before starting the app. The Compose data-prep helper
will create any missing `documents/` or `tmp/` directories and adjust ownership
for the configured user.

## JSON Export Is Not a Full Restore Backup

The app currently has a Backup page at:

```text
/backup
```

That page offers:

```text
/export.json
```

This JSON export is useful for reading, auditing, migration work, or keeping an
extra copy of application, document, and contact records. It is not yet a full
restore mechanism for the running app.

For now, use data directory backups for disaster recovery, moves, and complete
restores.
