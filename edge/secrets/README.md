# Edge secrets

Everything in this folder except this README is git-ignored. Create these files on the Pi only:

| File | What | How |
|---|---|---|
| `pg_password.txt` | Postgres password | `openssl rand -base64 32 > pg_password.txt` |
| `ingest_hmac_key.txt` | Key the phone uses to sign events | `openssl rand -base64 32 > ingest_hmac_key.txt` (also stored in the Android Keystore) |
| `client-ca.crt` | Private CA that signs the phone's client cert | Generated in phase 2 (CA key stays **offline**, not on the Pi) |
| `iot-device.crt` / `iot-device.key` | AWS IoT Core device identity | Created by Terraform in phase 4 |

Permissions: `chmod 600` on every file. The Go containers run as uid 65532 (distroless `nonroot`), so
make the files readable by that uid: `sudo chown 65532:65532 *.txt *.crt *.key`. Postgres reads
`pg_password.txt` as root during init.
