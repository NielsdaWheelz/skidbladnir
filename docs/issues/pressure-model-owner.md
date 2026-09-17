# pressure contracts mixed into the product model

problem: `ProductModel.kt` mixes pressure's domain/wire types, decoding,
validation, and stale-state handling with unrelated session, directory, and
terminal contracts. pressure already has separate presentation and rendering.

resolved when: give the pressure contract one cohesive source file, retaining
existing APIs and behavior. move its two shared JSON object helpers to the
existing strict JSON owner, and leave dashboard-scope policy in presentation.
characterize response decoding through fresh/stale presentation, preserving
metric availability, history, invalid-response handling, and timestamp admission.
