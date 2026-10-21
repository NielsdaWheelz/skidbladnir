# cancelled relay and runtime-proof cloud resources

problem: the superseded relay design and the required two-project runtime proof
created google resources. none is a skid-operated production backend. the
`skid.nielseriknandal.com` deployment is CANCELLED; no dns, caddy or sender service
deployment is required or authorized by the revised source change.

impact: unused projects, apps and service-account keys remain operator-owned
maintenance/security obligations. do not delete them as incidental source cleanup.
the authorized installed setup deliberately reuses the original project,
canonical production app and dedicated sender for this owner's optional push.
no public installer silently selects them. retain those production dependencies;
unused qualification resources remain separate cleanup.

live read-only inventory, 2026-10-20 (02:50 utc on2026-10-21):

| resource | known state |
| --- | --- |
| project `skidbladnir-push`, number `905675056539` | retained for owner-configured personal push; no sender deployment |
| android app `1:905675056539:android:d46a5410aee21ee740ed3e` | retained canonical production package; final apk has no embedded project identity |
| service account `skid-push-sender@skidbladnir-push.iam.gserviceaccount.com` | retained dedicated sender; one active user key matches private export, custom role `skidPushSend` grants only `cloudmessaging.messages.create` |
| android app `1:905675056539:android:65a1eb354627cf2640ed3e` | isolated runtime-proof package registration |
| project `skid-runtime-proof-20261020`, number `649417527697` | temporary second-project feasibility proof |
| android app `1:649417527697:android:3ae3575430dc0ba855ca30` | same isolated runtime-proof package |
| service account `skid-push-proof@skid-runtime-proof-20261020.iam.gserviceaccount.com` | one generated private key; custom role `skidPushProofSend`, message-create only |
| automatically created browser keys | no caller in admitted skid configuration; verify wider ownership before changing provider-generated resources |
| firebase-managed `firebase-adminsdk` accounts | no user-managed keys; provider-managed roles are separate from skid sending permission; retain unless explicitly proven unused |
| firebase client keys/API enablement | client keys restricted to firebase APIs; installations, registration and messaging APIs enabled on both projects |
| `skid.nielseriknandal.com` | dns-only A record was proposed, not installed by this work; read-only A lookup on 2026-10-20 returned no answer |
| devbox caddy/sender | candidate caddy syntax checked only; installed config unchanged; no sender service, unix user or public endpoint installed |

private material is outside source: original key/public export under
`~/.config/skidbladnir/firebase/`; proof key/config/receipts under
`~/.local/state/skidbladnir-runtime-push-proof/`. prepared local deployment bundle
and `afa51ca` release candidate under the private cutover directory are marked
cancelled/superseded. their prior validation is historical, not activation approval.
no secret values belong in this issue.

resolve separately after installed acceptance: retain the canonical production
project/app/sender/client configuration; obtain explicit cleanup approval for
the unused runtime-proof registration inside that project and the separate
temporary proof project/app/sender/key. verify current apps, IAM bindings, keys
and any owner-created DNS record immediately before removal. revoke
unused private keys and remove owned obsolete apps/projects/DNS only with explicit
cleanup authorization. remove corresponding private local artifacts, then verify
no active optional integration depends on them. preserve release-signing keys and
unrelated cutover converters. record what was deleted and what remains.
