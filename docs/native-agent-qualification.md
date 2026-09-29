# native interaction qualification

2026-09-28 · isolated source qualification; no fleet deployment or release.
2026-09-29 delivery changes the helper source/lock and codex bootstrap. the
disposable frozen helper installation and mac codex bootstrap now pass; scoped
linux apparmor namespaces pass with a temporary profile. installed policy,
full linux provider control and coordinated cutover remain `NOT_RUN`. the dated
evidence below does not qualify those remaining boundaries.
[the contract](native-agent-observation.md) owns requirements and accepted costs.
the [claude recipient defect](issues/native-message-qualification.md) makes native
input unavailable under the contract; status/history/results and matched
background stop remain supported.

## boundaries exercised

| boundary | evidence |
| --- | --- |
| codex selected view | actual patched tuis and direct unix app-server: two owners, new/clear, cached resume a→b→a, overview, fork, child/parent, process/server restart; owner pid/birth checks, invalidation ordering, reconnect revocation and unsupported-owner rejection |
| native controls | actual provider/helper/gateway: peer/user submission, working/approval/failure, persistent user queue and restart, exact interruption; stale turn cannot cancel its successor; reads never resume saved conversations. actual arch claude background workers: exact stop, wrong-birth rejection, independent control survives and saved history remains |
| native history | actual provider plus helper: 129-turn paging, uuidv7/timestamp/final-answer eligibility, fresh unmaterialized history, cursor invalidation; sdk assistant-block grouping and finalized claude reply identity |
| lifecycle and wait | actual public start, retained-terminal stop and compound close; saved history and codex queue survive closure; bounded waits cover idle, timeout, cancellation, unavailable and identity changes |
| close authority | actual tmux/kernel regression: a live captured agent backgrounded behind a replacement foreground process rejects closure; only proven exit permits the surviving shell to close |
| machine composition | actual mac and arch gateways over reciprocal ssh forwarding; current arch cli→mac codex admission and finalized reply recovery; current claude sends reject `unavailable/not_sent`, while separate explicit terminal prompting produces a saved native reply |
| desktop unread | real browser→tls gateway→pty→isolated tmux first-hello; later reply stays unread, failed attach/reconnect cannot acknowledge it; independent-process file merges, interrupted baselines and unavailable storage/history |
| android unread | owned emulator with actual controller, tls/wss and datastore: restart, offline recovery, captured-only acknowledgement, same-pane shell, hiding/deletion/reappearance, partial pages, fair scans and failed writes |
| content | desktop 80×24 and android 320dp review: working plus unread, previous-agent badge, merged accessibility, no reasons/inference or false outcome copy |
| native installation | real immutable codex source/patch/locked release build and generation installation; execution under a second uid; socket root/granted inode/private locks, restart repair and long rendezvous aliases on linux/darwin. final checksummed helper patch, frozen uv/python/sdk installation, private generation, launcher and exact native shim passed |
| coexistence | owned herdr pane used the patched provider and produced a native finalized reply; no user pane or account was retargeted |
| claude recipient | native provider FAIL: a held old-session message approved after `/clear` appears in successor sdk history. product red→green: peer/user/queue sends reject before provider discovery, no unsafe adapter remains, reads/result ids stay intact |

meaningful red→green repairs included strict nullable provider schema, selected-view
ordering, exact native error certainty, history materialization, linux native
identity format, long socket aliases, android schema validation/fair scans/whole-call
timeouts, truthful close copy, foreground closure authority and strict send
acceptance receipts. native claude recipient safety remains unsupported. a custom-function-hook barrier was `NOT_RUN`;
the supported command-hook barrier reached the actual failure. a herdr wait
timed out; only the independently finalized native reply is qualified.

## limits and delivery

2026-09-29 source follow-up: the exact merged helper commit and updated lock
passed a fresh frozen install and repeat in an empty disposable home; python,
uv, sdk, shim and launcher matched their pins. the final pinned codex package
started a native daemon from a marked mac tui; a second tui reused the same
owner and preserved the first selected view. read-only observation succeeded;
spoofed-owner registration and cross-view control failed. the upstream update
marker stayed absent. the debug daemon used about 188 mib resident in that
fixture; all test-owned processes, sockets and files were removed. ubuntu noble
installed x86_64 apparmor, coordinated package installation, installed-fleet behavior
and physical-phone usability remain unqualified. [release qualification](issues/native-agent-release-qualification.md)
tracks that cutover.

2026-09-29 ubuntu noble x86_64 follow-up: ordinary-user system bubblewrap
passed namespace creation; a byte-identical copy at the native package path
failed without the proposed profile. a uniquely named temporary profile with
the declared attachment/body made both package paths pass user, network and
pid namespace creation. the profile and all test-owned files were removed.
the persistent policy was absent; its installation and full linux provider
control were `NOT_RUN` in this namespace check.

2026-09-29 coordinated mac follow-up: the exact dev-server installer built the
pinned codex release package and repeated without changing its installed
marker. published skid v0.10.4, the frozen merged helper and that release
binary passed isolated selected-view binding, native user send, bounded
finalized reply read with matching turn/output identity, and stale-process
rejection against a controlled local model endpoint. exact native stop and
fixture cleanup passed. this does not qualify authenticated model access,
installed hosts or the physical phone.

2026-09-29 linux provider follow-up: a disposable ubuntu noble x86_64 package
built the exact pinned codex cli and code-mode helper. marked tuis shared one
native owner with distinct selected views; guarded send and finalized read
passed against a controlled local model endpoint, while spoofed and stale
controls failed. package and daemon-copy bubblewrap were denied without the
temporary apparmor policy, allowed with it, then denied after removal. native
stop, policy removal and fixture cleanup passed. this was a debug build with
code-mode runtime disabled; no linux installer release build, installed policy,
authenticated access or fleet behavior was tested.

codex execution used a controlled model http endpoint with actual provider,
history, queue and tui processes. it does not qualify the real openai backend.
claude used its actual native binary and sdk with a real account. the emulator
does not establish physical-phone usability or installed-fleet acceptance.

the helper source and lock pin are authoritative in skid's
`deployment/native-control/pin.json` and dev-server's
`assets/skid-provider/native-control.json`; dev-server's
`assets/codex/native-source.json` owns the codex source and patch digest.
skid engineering checks (`scripts/check verify`) passed after temporary test deletion.
helper lint/format/types and installer shell checks passed. task-added tests,
fixtures, test trust entries and owned processes are removed; existing upstream
tests remain. test deletion deliberately leaves
no new regression protection.
ship the coordinated source/contracts/pins together; rollback uses the prior
complete release. v0.10.4 publication is complete; fleet installation is separate.
