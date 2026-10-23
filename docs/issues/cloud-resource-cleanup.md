# unused cloud resources

problem: the cancelled relay design and a two-project runtime proof created
google resources. none backs a skid-operated service. the owner may reuse the
original project for personal opt-in push; no installer or public config selects
it.

impact: unused projects, apps and service-account keys remain operator-owned
security obligations. do not delete them as incidental source cleanup.

inventory, 2026-10-20:

| resource | known state |
| --- | --- |
| project `skidbladnir-push`, number `905675056539` | created for the relay; no sender deployed |
| android app `1:905675056539:android:d46a5410aee21ee740ed3e` | production package registration |
| service account `skid-push-sender@skidbladnir-push.iam.gserviceaccount.com` | one generated key; custom role `skidPushSend`, `cloudmessaging.messages.create` only |
| android app `1:905675056539:android:65a1eb354627cf2640ed3e` | runtime-proof package registration |
| project `skid-runtime-proof-20261020`, number `649417527697` | temporary second-project proof |
| android app `1:649417527697:android:3ae3575430dc0ba855ca30` | same runtime-proof package |
| service account `skid-push-proof@skid-runtime-proof-20261020.iam.gserviceaccount.com` | one generated key; custom role `skidPushProofSend`, message-create only |
| firebase client keys and apis | keys restricted to firebase apis; installations, registration and messaging apis enabled on both projects |
| `skid.nielseriknandal.com` | no record found by a read-only lookup |

private material is outside source: `~/.config/skidbladnir/firebase/` (original
key and public export) and `~/.local/state/skidbladnir-runtime-push-proof/` (proof
key, config, receipts), both present on 2026-10-22, plus a cancelled deployment
bundle and release candidate (`afa51ca`) in the private cutover directory. no
secret values belong here.

resolved when: with explicit cleanup authorization, the owner inventories current
apps, iam bindings, keys, api keys and any dns record, decides reuse or removal,
revokes unused keys, removes obsolete apps, projects, dns and the matching local
artifacts, confirms no optional integration depends on them, keeps release-signing
keys, and records what remains.
