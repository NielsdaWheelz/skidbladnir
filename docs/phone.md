# Set up the Android app

> This page is written from the code. Neither installing from a phone's browser
> nor installing through Obtainium has been walked step by step.

The app is not on Google Play. Each [release](https://github.com/NielsdaWheelz/skidbladnir/releases)
carries it as `skidbladnir-android.apk`, built from the same source as that
release's computer archives.

You need:

- Android 13 or later with Google Play services, which pairing uses to scan the
  QR code and push notifications use for delivery;
- the Tailscale app, signed in to the same tailnet as your computers;
- at least one computer [connected](install.md#connect-the-first-computer);
- the APK from the release your computers run.

## Check the signer

Do this before the first install. Afterwards Android itself refuses any update
signed by another key.

1. On a computer, download the APK and the release's checksums, the release
   public key from the repository at the same tag, and verify them. Releases
   sign their checksums from v0.23.0:

   ```sh
   tag=vX.Y.Z
   base=https://github.com/NielsdaWheelz/skidbladnir/releases/download/$tag
   curl -fsSL -O "$base/skidbladnir-android.apk" -O "$base/android-signing-cert.sha256" \
     -O "$base/SHA256SUMS" -O "$base/SHA256SUMS.sig"
   curl -fsSLo release-public-key.pem \
     "https://raw.githubusercontent.com/NielsdaWheelz/skidbladnir/$tag/internal/install/release-public-key.pem"
   openssl dgst -sha256 -binary -out SHA256SUMS.digest SHA256SUMS
   openssl pkeyutl -verify -rawin -pubin -inkey release-public-key.pem \
     -in SHA256SUMS.digest -sigfile SHA256SUMS.sig
   sha256sum -c --ignore-missing SHA256SUMS    # macOS: shasum -a 256 -c --ignore-missing SHA256SUMS
   ```

   Expect `Signature Verified Successfully`, then `OK` for
   `android-signing-cert.sha256` and `skidbladnir-android.apk`. OpenSSL 3 or
   later is needed; on macOS use `"$(brew --prefix openssl@3)/bin/openssl"` if
   `openssl` is the system one.

2. Print the APK's signer with `apksigner`, from the Android SDK build-tools
   (Debian and Ubuntu also package it as `apksigner`):

   ```sh
   apksigner verify --print-certs skidbladnir-android.apk | grep 'certificate SHA-256'
   cat android-signing-cert.sha256
   ```

   There must be exactly one line, `Signer #1 certificate SHA-256 digest: ...`,
   and its value must equal the contents of `android-signing-cert.sha256`. The
   same value is the repository's pin,
   [`android/app-signing-cert.sha256`](../android/app-signing-cert.sha256), and
   the release notes' "Android signer SHA-256".

3. Install this file: copy it to the phone, or use `adb` (below).

The release key that signs `SHA256SUMS` is the one that signs the computer
archives and is written into the installer.

## Install

By hand:

1. Copy the checked file to the phone. Downloading `skidbladnir-android.apk`
   again from the release page on the phone also works, but then the check above
   covers the release's file, not the bytes on the phone, and Android pins
   whatever signer they carry for every later update.
2. Open the file. When Android asks, allow your browser or file manager to
   install unknown apps.
3. Google Play Protect may warn about an app it does not know. Install it anyway
   if you checked the signer.

With `adb` from a computer, `adb install -r skidbladnir-android.apk` installs or
updates in place and keeps the app's data. From a checkout of this repository,
`scripts/install-android skidbladnir-android.apk` first checks the signer against
the pin and the package name; it needs the Android SDK build-tools 36.1.0 and
exactly one authorized device.

### Obtainium

[Obtainium](https://github.com/ImranR98/Obtainium) installs and updates apps from
their GitHub releases.

1. In Obtainium, add an app with the source URL
   `https://github.com/NielsdaWheelz/skidbladnir`. Each release has one APK.
2. Install it from Obtainium, which then offers each new release.

Keep the phone on the release your computers run: Obtainium may offer a release
before you have updated them.

## Pair

1. In the Tailscale app, sign in to your tailnet and connect. The app's
   **Install Tailscale** or **Open Tailscale** button takes you there.
2. On a computer in the fleet, run `skid connect`. It prints a QR code, valid for
   one use within five minutes.
3. Open Skíðblaðnir, tap **Connect** and scan the QR code.
4. The dashboard lists the tmux sessions of every computer in the fleet.

A paired phone can open a shell on every computer in the fleet. It cannot
invite or remove devices or apply configuration; computers do that
([trust](architecture.md#3-topology-and-trust)). To revoke a phone, run
`skid remove DEVICE_ID` on a computer; `skid status` lists device IDs.

If pairing fails, for example because the code expired or was used, the app
says "Couldn’t connect this phone. Scan a fresh fleet invite to retry.": run
`skid connect` again and scan the new code. A phone paired to one fleet refuses
another fleet's invitation; to switch fleets, clear Skíðblaðnir's storage in
Android's app settings and pair again. Pairing needs the QR scanner; there is no
way to paste an invitation.

## Update

1. Download the APK of the release your computers now run.
2. Install it over the installed app, in any of the ways above. The app keeps
   its data and its pairing, because the signer is the same.

Android refuses an update signed by another key and refuses a downgrade. In
either case, uninstall the app, which deletes its pairing, install the new APK,
pair again, and remove the old phone entry with `skid remove DEVICE_ID`. The
signer changed in v0.23.0, so a phone running v0.22.0 or earlier must take this
path once.

## Push notifications (optional)

Without push, everything works except background notifications on the phone.
With it, the hub sends a content-free wake-up message through your own Firebase
project straight to Google; the phone then reads what changed from the hub over
your tailnet. No text, terminal data or fleet identity passes through Google, and
skid runs no hosted notification service. The contract is
[Android push](architecture.md#android-push).

Set it up once:

1. In the Firebase console, create a project and add an Android app with the
   package name `dev.niels.skidbladnir`. Download its `google-services.json`.
   Phones keep using the released APK; nothing is rebuilt.
2. In Google Cloud, for that project, enable the Firebase Cloud Messaging API
   (HTTP v1), and restrict the project's Android API key to the Firebase APIs it
   needs.
3. In Google Cloud, create a dedicated service account with a custom role that
   holds only `cloudmessaging.messages.create`, and export a JSON key for it.
4. As the user who runs skid on the hub, install the key with the file-copying
   `install` command:

   ```sh
   install -m 600 KEY.json "${XDG_CONFIG_HOME:-$HOME/.config}/skidbladnir/android-push-credentials.json"
   ```

   It must be a regular file, not a link, of mode `0600`, at most 64 KiB, whose
   `project_id` is the Firebase project's. Never put it in `config.yaml`, in a
   command's arguments, on a phone or in a repository.
5. Add the public client options to the hub's `config.yaml`, from the
   `google-services.json` client whose package is `dev.niels.skidbladnir`:

   ```yaml
   android_push:
     project_id: your-firebase-project                      # project_info.project_id
     application_id: '1:123456789:android:0123456789abcdef'  # client[].client_info.mobilesdk_app_id
     sender_id: '123456789'                                 # project_info.project_number, quoted
     api_key: your-public-android-client-key                # client[].api_key[].current_key
   ```

   Then run `skid apply`. Paired phones receive the options with the fleet.
6. On the phone, the dashboard shows a notice, such as "notifications are
   blocked in system settings." or "notification setup required.", with a
   **set up notifications** button. Tap it, tap **set up notifications** in the
   dialog, and allow notifications when Android asks.
7. Confirm one real notification: put the app in the background on the phone
   and let an agent ask for input. Pairing, or Google accepting a message, does
   not prove delivery.

Afterwards:

- Rotate the key: replace the file (mode `0600`), run `skid apply`, then revoke
  the old key in Google Cloud.
- Turn push off: remove `android_push` from `config.yaml` and run `skid apply`.
- After the public options change, the phone shows "stop and reopen skid to
  activate changed notification settings." Force stop Skíðblaðnir in Android's
  app settings and reopen it; swiping it away may leave it running.
- If the phone shows "configure android push credentials on the hub.", the hub
  has no usable key file: check step 4 and run `skid apply`.
- Before moving the hub with `skid recover`, place a matching key on the new hub.
