# Backup and Restore

JobHunt OS stores its durable application data in the configured data directory.
For the recommended Docker Compose install, that directory is `./data` next to
`docker-compose.yml` inside `deploy/`.

That directory can contain application notes, contacts, recruiter messages,
document metadata, uploaded files, and the SQLite database. Treat backups as
sensitive personal data.

## Back Up a Docker Compose Install

From the directory that contains `docker-compose.yml`:

```sh
docker compose down
tar -czf jobhunt-os-backup-$(date +%Y%m%d).tgz data
docker compose up -d
```

Stopping the container before archiving keeps the SQLite database and local
files in a quiet state while the backup is created. The archive records the
current file permissions; the restore examples below use `tar -xpf` so those
permissions are applied when extracting.

Store the resulting `.tgz` outside the install directory, such as on a different
disk or backup service. Prefer encrypted storage for any long-lived backup.

## Encrypt Backups

The simplest default flow is still: stop Compose, archive `./data`, start
Compose again, then move the backup somewhere safe. For extra protection, encrypt
the archive before copying it to cloud storage, removable media, or another
machine.

Common options include:

- Full-disk or external-disk encryption from the operating system.
- Encrypted backup tools such as Restic, Borg, or Kopia.
- A password-encrypted file using a tool such as GnuPG or age.
- An encrypted folder or bucket managed by the storage provider.

No paid service is required. Pick a tool you can restore from confidently, keep
the password or key outside the JobHunt OS install directory, and test recovery
before relying on it.

Example with GnuPG:

```sh
gpg --symmetric --cipher-algo AES256 jobhunt-os-backup-YYYYMMDD.tgz
```

Replace `YYYYMMDD` with the date in your backup filename. This creates
`jobhunt-os-backup-YYYYMMDD.tgz.gpg`. Store the encrypted file and
protect the passphrase. If you keep the unencrypted `.tgz`, protect it with the
same care as the live `./data` directory.

Example with age password encryption:

```sh
age -p -o jobhunt-os-backup-YYYYMMDD.tgz.age jobhunt-os-backup-YYYYMMDD.tgz
```

This creates `jobhunt-os-backup-YYYYMMDD.tgz.age`.

## Restore or Move an Install

To restore or move JobHunt OS to another machine:

1. Install Docker and Docker Compose on the target machine.
2. Clone the repository or copy the `deploy/` directory to the target machine.
3. Decrypt the backup if it was encrypted.
4. Stop JobHunt OS if it is already running.
5. Copy or extract the backed-up `data/` directory into the same directory,
   preserving file permissions.
6. Restore or recreate `deploy/.env` and `deploy/.secrets/admin-password`.
7. Start the app from `deploy/`.

For a plain tar archive, replace `YYYYMMDD` with the date in your backup
filename:

```sh
docker compose down
tar -xzpf jobhunt-os-backup-YYYYMMDD.tgz
docker compose up -d
```

For a GnuPG-encrypted archive:

```sh
docker compose down
gpg -o jobhunt-os-backup-YYYYMMDD.tgz -d jobhunt-os-backup-YYYYMMDD.tgz.gpg
tar -xzpf jobhunt-os-backup-YYYYMMDD.tgz
docker compose up -d
```

For an age-encrypted archive, use `age -d` in place of the `gpg` command:

```sh
age -d -o jobhunt-os-backup-YYYYMMDD.tgz jobhunt-os-backup-YYYYMMDD.tgz.age
```

Then visit:

```text
http://127.0.0.1:8080
```

If you are restoring from the tar archive created above, the install directory
should end up with this shape:

```text
jobhunt-os/
  deploy/
    docker-compose.yml
    .env
    .secrets/
      admin-password
    data/
      jobhunt-os.db
      documents/
      tmp/
```

When restoring on the same Linux host, extracting as root with `tar -xzpf` keeps
the archived numeric owner IDs and file modes. On another host, the old numeric
UID/GID may belong to a different user. In that case, create or update `.env`
before starting the app:

```text
JOBHUNT_UID=<host-uid>
JOBHUNT_GID=<host-gid>
```

Then run `docker compose up -d`. The Compose data-prep helper will create any
missing `documents/` or `tmp/` directories and adjust ownership for the
configured user while preserving normal private file permissions.

## JSON Export Is Not a Full Restore Backup

The Settings page includes a JSON export control:

```text
/settings
```

The export endpoint is:

```text
/export.json
```

This JSON export is useful for reading, auditing, migration work, or keeping an
extra copy of application, document, and contact records. It is not yet a full
restore mechanism for the running app.

The export contains sensitive job-hunt data, including application notes,
timeline entries, contacts, and document records. Store it carefully, do not
commit it to the repository, and prefer encrypted storage when keeping it
outside your machine.

The legacy `/backup` route redirects to `/settings`.

For now, use data directory backups for disaster recovery, moves, and complete
restores.
