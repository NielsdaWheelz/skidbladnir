# optional android background notifications

status: target setup. isolated [qualification](https://github.com/NielsdaWheelz/skidbladnir/blob/12b15163d9faf529416d486e0156b8b83821fa4a/docs/generalization-qualification.md)
records runtime feasibility; the installed round trip remains [unrun](issues/unrun-acceptance.md).

skip this guide to use desktop/mobile session management without firebase.
mac notifications also work independently. skid runs no notification backend.
choosing android push means owning a firebase project, sending credentials and
their maintenance. the existing hub sends directly to google; phones fetch
notification details from their hub over tailscale. no additional server,
domain, dns, caddy configuration or notification app.

## set up once on the hub

1. create your own project in the [firebase console](https://console.firebase.google.com/).
   add an android app with package `dev.niels.skidbladnir`. download its
   `google-services.json`; you will copy four PUBLIC values into yaml below.
   install the ordinary distributed skid apk. there is no custom apk/build step.
2. enable the firebase cloud messaging http v1 api. ensure firebase installations
   and fcm registration APIs are enabled and allowed by the android client key's
   API restrictions. keep the key restricted to firebase APIs; it is public
   client configuration, not permission to send messages.
3. create a dedicated service account in this same project. grant a custom
   project role containing only `cloudmessaging.messages.create`, then generate
   its json key. do not grant owner/editor or use an unrelated admin account.
   [fcm authorization](https://firebase.google.com/docs/cloud-messaging/send/v1-api)
   and [iam permissions](https://docs.cloud.google.com/iam/docs/roles-permissions/firebasecloudmessaging)
   describe the provider boundary.
4. put that PRIVATE, unmodified google service-account json export on the hub,
   as its normal login user:

   ```sh
   mkdir -p "${XDG_CONFIG_HOME:-$HOME/.config}/skidbladnir"
   chmod 700 "${XDG_CONFIG_HOME:-$HOME/.config}/skidbladnir"
   install -m 600 /path/to/downloaded-service-account.json \
     "${XDG_CONFIG_HOME:-$HOME/.config}/skidbladnir/android-push-credentials.json"
   ```

   keep the original key secure. do not paste it into yaml, command arguments,
   phone configuration, a repository or an issue. skid reads this exact local
   file; it does not discover an ambient google account.
5. add the public fields to the hub's existing `config.yaml`:

   ```yaml
   android_push:
     project_id: your-firebase-project
     application_id: '1:123456789:android:0123456789abcdef'
     sender_id: '123456789'
     api_key: your-public-android-client-key
   ```

   replace every example value. select the client entry whose android package
   is `dev.niels.skidbladnir`:

   | yaml field | `google-services.json` value |
   | --- | --- |
   | `project_id` | `project_info.project_id` |
   | `application_id` | matching `client[].client_info.mobilesdk_app_id` |
   | `sender_id` | `project_info.project_number`, as a quoted string |
   | `api_key` | matching `client[].api_key[].current_key` |

6. run `skid apply`. pair normally with `skid connect` and allow android
   notifications. signed fleet state transfers public configuration
   automatically. already paired phones learn it through normal reconciliation;
   private credentials stay on the hub.

the hub needs outbound https to google. the supported phone needs google play
services, notification permission and tailscale reachability to the hub.
successful pairing or fcm request acceptance does not prove background delivery;
verify a real current notification while the phone is backgrounded.

## changes and opting out

omit `android_push` and run `skid apply` to disable background push. this is
healthy operation: no firebase initialization/registration or recurring setup
error. session management, mac notifications and acknowledgement memory remain.
existing notification channels and posted notices are not reset.

to rotate the sending key, replace the private file with another matching
service-account export, retain mode0600, and apply. public configuration stays
the same, so phones need no new build or pairing. revoke the old key in google
after confirming the replacement works.

changing public firebase options requires one phone app restart. skid fences
old push immediately and preserves sessions/acknowledgements. when prompted,
stop skid in android's app settings and reopen it; merely swiping away a task
may leave its process alive. same-project re-pairing uses independent fleet
credentials without resetting notification memory. no app-data reset.

before moving the hub with push configured, securely place a matching private
credential file on the destination. skid never transfers that secret. missing
credentials leave placement pending on the old hub. alternatively disable push,
move, then configure it on the new hub. ordinary fcm outages affect push alone.

## status and recovery

| status | action |
| --- | --- |
| disabled | none; push is optional |
| unconfigured | finish public configuration and the hub's private credential file, then apply |
| permission blocked | enable notifications/google play services on the phone |
| restart required | stop and reopen skid to activate changed firebase options |
| pending | keep skid open and networking available until registration and possession proof complete |
| unavailable | check hub credential access and outbound google connectivity, then retry through ordinary apply/resume |

snapshot access never clears a separate push transport failure. only unseen,
current attention admitted by the device can produce a notice. force-stopping
android suppresses push until reopening; delivery before first unlock is not
supported. users remain responsible for project/key access and maintenance.
