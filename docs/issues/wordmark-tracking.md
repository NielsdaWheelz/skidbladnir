# The wordmark's tracking differs between the spec and the app

Problem: design language §9 sets the wordmark in capitals with +4% tracking.
The app's wordmark on the connect screen (`FleetConnectScreen.kt`) uses
`headlineLarge` with no added tracking.

Impact: the app and anything drawn from the spec, such as the site and social
card, set the name differently.

Evidence: found while making `site/assets/social-card.png` on 2026-10-26, which
follows §9.

Resolved when: the app and §9 agree.
