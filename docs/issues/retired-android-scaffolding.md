# android scaffolding after test retirement

problem: `deviceDebug` and its signing/source-set admission remain in
`android/app/build.gradle.kts` without an executable caller. `ui-tooling-preview`
has no preview consumer. `retainedWorkingDirectoryView` in
`WorkingDirectoryPicker.kt` has no caller. `scripts/build` exports two version
environment variables that have no reader; gradle properties own those values.

impact: unused build variants, dependencies and helpers obscure the supported
development surface.

resolved when: remove unused surface and verify ordinary debug/release task
admission, build and lint. preserve the debug seal gallery. removing
`deviceDebug` deliberately removes the signed debug installation capability;
no current workflow uses it. no phone operation is needed to prove dead-code
or build-configuration removal.
