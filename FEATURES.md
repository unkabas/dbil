# Features — how to use dbil

User-facing guide for the day-to-day workflows. For architecture see the
knowledge graph (`.understand-anything/`), for the security model see
`SECURITY.md`.

## Roles & access

Every account has one of three roles. The first admin is created on `dbil init`
with a random password printed to the console (and `initial-credentials.txt`).

| Role     | Browse schema / data / observability | Edit data & run DML | Manage connections & SSH hosts | Manage users |
| -------- | :----------------------------------: | :-----------------: | :----------------------------: | :----------: |
| `admin`  | ✓ | ✓ | ✓ | ✓ |
| `member` | ✓ | ✓ | ✓ | — |
| `viewer` | ✓ | — | — | — |

A `viewer` is strictly read-only: write SQL in the editor and inline grid edits
are rejected with `403`.

## Managing users (admin only)

Open the **Users** tab (visible only to admins).

- **Create user** — enter an email and pick a role. dbil generates a strong
  one-time password and shows it once; copy it and hand it to the person.
- **First login** — a newly created user (and anyone whose password an admin
  reset) is forced to choose a new password before they can use the app.
- **Change role / reset password / delete** — inline on each row. The last
  remaining admin cannot be demoted or deleted, and you cannot delete yourself.
- **Change your own password** — via the forced-rotation dialog, or by an admin
  reset.

## Editing data (DataGrip-style)

In the **Data** tab, tables that have a **primary key** are editable by
`admin`/`member`. Views and PK-less tables stay read-only.

- **Edit a cell** — double-click it, type, press `Enter` to stage the change
  (`Esc` cancels). Staged cells are tinted with the accent colour.
- **Set NULL** — while editing a cell press `Ctrl`/`Cmd`+`Backspace`.
- **Delete a row** — click the trash icon in the row gutter; the row is struck
  through. Click again to undo.
- **Add a row** — click **Row** in the toolbar to append a draft row, fill the
  cells you want (leave a cell blank to use the column default), and discard it
  with the `✕` in its gutter.
- **Apply or revert** — pending edits accumulate; the floating bar shows the
  count. **Submit** applies the whole batch in a single transaction (all or
  nothing); **Revert** discards everything.

Tag policy still applies: a `staging` connection asks you to confirm, and a
`production` connection requires confirmation (and a passphrase if the
connection is passphrase-protected). Dangerous statements stay blocked on
`production` — but inline edits are always scoped to the primary key, so they
are never "dangerous".

## Connecting through an SSH tunnel

To reach a database whose ports are firewalled, tunnel through a bastion.

1. **Connections → SSH tunnels → Add SSH host.** Give it an alias, the bastion
   host/port, the SSH username, and either a **private key** (PEM/OpenSSH, plus
   its passphrase if the key is encrypted) or a **password**. Optionally protect
   the stored secret with an at-rest passphrase (recommended for production).
2. **Test** the host. The first successful test pins the server's host-key
   fingerprint; later connections fail if the fingerprint ever changes.
3. **Create a connection** and pick the SSH host under **SSH tunnel**. Set the
   connection's host/port to the database address *as seen from the bastion*
   (often `localhost:5432`). dbil routes every backend connection through the
   tunnel.

Secrets (SSH keys/passwords) are encrypted at rest with the same envelope used
for connection credentials; passphrase-wrapped hosts require the passphrase to
open the tunnel.
