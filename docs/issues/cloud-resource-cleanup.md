# Unused cloud resources

Problem: the cancelled relay design and a two-project runtime proof created Google
Cloud resources: two projects, three Android app registrations, two service
accounts with one generated key each, and Firebase client keys restricted to
Firebase APIs. None backs a skid-operated service. The owner may reuse the
original project for personal opt-in push; no installer or public configuration
selects it.

Impact: unused projects, apps and service-account keys remain operator-owned
security obligations. Do not delete them as incidental source cleanup.

The inventory of 2026-10-20, with identifiers and the local paths of private
material, is kept in the owner's private notes. No identifiers or secret values
belong here.

Resolved when: with explicit cleanup authorization, the owner inventories current
apps, IAM bindings, keys, API keys and any DNS record, decides reuse or removal,
revokes unused keys, removes obsolete apps, projects, DNS and the matching local
artifacts, confirms no optional integration depends on them, keeps release-signing
keys, and records what remains. Git history is not rewritten: the Firebase client
key committed in `android/app/firebase.json` from `b5189e1` to `f21112c` is
handled in Google Cloud, and GitHub secret-scanning alert #1, which the owner
left open, closes as revoked if the key is deleted or as won't fix if it is
kept.
