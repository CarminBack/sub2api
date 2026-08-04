# Token3 Managed Updater

The Token3 admin update button writes a request to
`/app/data/token3-update/request.json`. A root-owned systemd path unit starts
the host updater, which merges the requested official stable tag into the
custom branch and deploys only after GitHub CI, security, and image workflows
succeed.

## Server installation

1. Add a dedicated writable deploy key to `CarminBack/sub2api`. Store the
   private key at `/root/.ssh/token3_updater_ed25519` with mode `0600`.
2. Install `deploy/token3-updater.sh` as
   `/usr/local/sbin/token3-updater` with mode `0755`.
3. Install both files from `deploy/systemd/` under `/etc/systemd/system/`.
4. Run `systemctl daemon-reload` and
   `systemctl enable --now token3-updater.path`.

Optional overrides belong in `/etc/token3-updater.env`. The production
defaults target `/opt/sub2api2`, Compose service `sub2api`, and container
`sub2api2`.

The updater uses a dedicated clone under `/opt/token3-updater`, rejects
prerelease/non-semver targets, never force-pushes, verifies all required
workflows, creates a validated PostgreSQL backup, and recreates only the
application service. Progress is written atomically to
`/opt/sub2api2/data/token3-update/status.json` for the original frontend UI.
