# nested sessions

product contract and acceptance record for quiet, exact session branches.
the [architecture](../architecture.md#nested-sessions) owns the implemented
interfaces. physical fleet and notification-center qualification remain open.

## creating a child

parent and child are tmux sessions. either can run a shell or an agent on any
enrolled machine. a session has at most one direct parent and can have children
of its own. identify each end by machine and session lifetime. rename, pane
switch, provider exit, reattach and gateway restart preserve the edge. group
membership is independent. existing sessions start at the top level. native
provider subagents stay inside their terminal. peer messages and jarvis job
assignments do not establish skid parentage. [session identity](../architecture.md#2-concepts)

- `skid start --delegate` uses the caller's verified skid session as parent.
  `--parent HANDLE` selects an existing fleet session explicitly. choose at most
  one. `--machine` selects the child's host. both forms support `--terminal`
  and `--profile`. only a profile accepts an optional initial brief.
  ordinary starts create top-level sessions.
- place an optional `parent` field below `group` in both creation menus. leave
  it blank even for “another like this”. search across the fleet, show name and
  machine, and bind the choice to its identity. if an explicit choice or the
  implicit caller cannot be resolved, or the parent is closing, refuse before
  creation. never silently create a top-level session instead.
- the child must already be nested on its first observation. creation and
  initial input have separate outcomes. if the brief fails after creation,
  return the exact created child and keep it nested.
- document the two options in `skid --help` and `skid help start`. the shared
  dev-server prompt in `assets/agent-instructions.md` should say “use
  `skid start --delegate` for sessions you supervise. use ordinary `skid start`
  for sessions the user will own.”

## status, silence and promotion

keep each child's actual status visible in its row and info. the hub creates
no human attention episode for a nested session. desktop and android exclude
it from `needs input`, its count and next/previous navigation. mac and android
suppress delivery and clear any obsolete notice. this includes ready,
permission, question, error and interruption. a child never changes its
parent's status or generates a proxy alert. [hub attention](../architecture.md#12-notifications) · [client queue](../architecture.md#views-and-the-needs-input-queue)

silence belongs to the session. inspecting it, changing its foreground or
losing contact with its parent cannot enable alerts. an explicit
`skid wait --state needs-input` still observes the terminal. a worker awaiting
approval can wait indefinitely. its orchestrator owns checking and escalation.

offer `make top-level` in the child's space/info view and phone session details,
including when its parent has disappeared. remove only the selected session's
parent edge. its descendants travel with it and stay silent. the promoted
session becomes eligible for new attention. use its current state as the
baseline so promotion never announces a backlog of child activity.

## browsing

branches start folded. in `all`, roots appear in their groups. reveal each
child once under its parent in stable sibling order, with its own machine.
group and machine filters keep matching children reachable even when a parent
falls outside the filter. show that parent as context. cli inventory continues
to enumerate every matching session.

- **desktop.** `space` opens the selected row's info. on a parent it also
  reveals direct children as ordinary selectable rows. down reaches the first
  child, enter attaches, and space on a child can open the next level. moving
  into a child keeps its ancestors expanded. space on an expanded parent folds
  the branch. a background update leaves the user's expansion and surviving
  selection alone. left/right and `h`/`l` switch views.
- **row.** the cursor occupies cell one, status two, and `▸` (folded), `▾`
  (expanded) or a blank (leaf) occupies three. leave cell four blank. the name
  lane starts at five, with child indentation inside that lane. status takes
  precedence at very narrow widths. show the direct-child count in info.
- **info.** show facts and actions. remove the screen excerpt, its cache, the
  `i` toggle back to the screen and the “read again” hint. opening, carrying
  the fold with tab or shift-tab, pressing ctrl-r, and a host becoming reachable
  cause **zero terminal reads for the fold**. remove its pending reads as well
  as its renderer. explicit `skid read`, attach and host status observation
  retain their own purposes. [fold controls](../../internal/sessionui/fold.go)
- **phone.** the card body attaches. a separate accessible control such as
  `3 children` opens the list. each child can be inspected or entered. back
  returns to the originating row. opening a disclosure acknowledges nothing.

## closing a branch

all skid close paths, including terminal-only close, obey the same rule.
closing a leaf uses the exact-target close routes. closing a parent requires
one confirmation for the whole branch. show the actual mode and ask, for example,
**“close parser rewrite and all 4 descendants on 2 machines?”** list every
target by name and machine, including folded grandchildren and filtered-out
sessions. cancel has no effect. to keep a subtree, promote its root first.
a noninteractive close of a parent reports that confirmation is required,
returns the proposed targets and closes nothing. confirmation binds the close
mode as well as the exact targets.

the parent host owns admission of direct children and must account for admitted
and pending children across gateway restarts. the child's host owns its tmux
session and exposes its nested status locally. the client addresses the
involved hosts directly. a scan of reachable hosts cannot prove completeness.
an unrelated offline machine need not block close. [host ownership](../architecture.md#3-topology-and-trust) · [mutation rules](../rules.md#mutations-delivery-and-retries)

1. capture the complete branch, its edges and each close target, including the
   pane when interrupting. wait for confirmation without holding locks. an
   unavailable branch host or unresolved creation prevents the first effect.
2. revalidate on confirmation. additions, promotions and replacements require
   fresh confirmation before acting on the changed branch. admission and
   promotion must be serialized with closing so a late child cannot slip
   through and a promoted child cannot be closed under its former parent.
3. close leaves first and the root last. each parent requires proof that its
   descendants closed and the checkpoint cannot restore them under that edge.
   on loss or uncertainty, stop dispatch. report each
   confirmed closure, remaining target and unknown outcome, including separate
   interrupt results when requested. a failure before root dispatch leaves the
   root open. a lost reply after root dispatch leaves its outcome unknown.
   never replay an uncertain effect.

a parent can disappear outside skid. show surviving children directly in
`all`, still silent and labelled `parent unavailable`. preserve nested status
through workspace recovery. reconnect exact edges when the old-to-new session mapping is
established. an unresolved edge stays visible and cannot authorize a destructive
close. a replacement with the same name inherits nothing. [workspace recovery](../session-recovery.md)

## acceptance

- create a shell, an agent and a cross-host grandchild through cli and picker.
  verify the chosen parents across rename and reattach, refusal for a stale
  choice, and the exact child receipt when initial input fails.
- expand, descend, attach, return and fold on desktop and phone. refresh an
  open desktop row, carry it with tab and restore host connectivity. observe
  zero fold reads while explicit cli read and attach work.
- run eight nested sessions through idle, approval, question, error and
  interruption. change the process in one. inspect their real states and
  observe zero child queue entries, mac alerts, android alerts or parent proxy
  alerts. exercise an ordinary top-level alert as the control.
- promote a child with a grandchild. only new work can alert for the promoted
  session. closing the former parent excludes both sessions.
- preview and cancel a cross-host close, then change the tree after preview.
  zero targets close on cancel or stale confirmation. fresh confirmation closes
  exactly the enumerated descendants before their root.
- race a new delegation with final close. either the branch accounts for the
  child or creation refuses. an offline branch host at confirmation means
  zero closes. losing it partway through produces an exact partial result
  without replay.
- drop the parent host, close a parent through native tmux and restore a
  workspace. surviving children stay discoverable and silent. only proven
  links are restored. a same-name replacement cannot adopt or close them.


## qualification and cutover

isolated verification uses separate tmux servers and loopback gateways, authored
provider frames, a private android emulator and mac notification reconciliation.
the enrolled fleet, actual mac notification-center delivery and production
firebase push remain unqualified. repeat those delivery cases with authorized
fixtures before calling rollout accepted.

upgrade gateways and clients together before creating nested sessions. older
gateways lack admission and close proofs. apply the dev-server instruction
after `skid help start` on each host documents `--delegate` and `--parent`.

a prepared close fence can survive lost connectivity. preserve the exact close
and release receipts. if release remains uncertain, inspect the host's
`session-branches-<machine>.json` and quiesce the operation before clearing an unspent
fence. a pending admission needs a proven child or a definitive no-create.
the system does not expire either record or retry a terminal effect.
