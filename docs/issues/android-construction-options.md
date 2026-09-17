# unused android construction options

problem: the controller, gateway client, and machine storage expose replacement
dependencies or storage names with no production callers. these options obscure
the application's fixed ownership and survived the test-harness retirement.

evidence: `MainActivity` constructs the controller with context and dashboard
entry only; the controller is the sole `GatewayClient` constructor caller;
`MachineStorage.production` is the only storage construction.

resolved when: remove unused constructor options and build the ordinary app.
preserve storage names, encryption, quarantine, commit rollback, transport
settings, and client shutdown. no alternate production implementation is needed.
