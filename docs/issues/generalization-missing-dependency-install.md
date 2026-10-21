# installation with missing shared dependencies

problem: actual fresh-user install/update/uninstall ran with tmux and tailscale
already installed. absent-package branches have source/isolated proof, not a
bare-os installation witness.

impact: do not claim the whole supported platform matrix was installed from
an os lacking both packages. ordinary installed service lifecycle is qualified.

evidence: [standalone qualification](../generalization-qualification.md) records
the locked-user/home/systemd lifecycle and three-host activation. shared packages
were preserved; removing them from the owner's active machines was not a test.

resolve: on disposable supported hosts with missing dependencies, run the public
verified installer, complete ordinary privilege/tailscale login steps, then
prove local attachment, service restart and owned uninstall. preserve shared
packages and unrelated routes; record each actual platform separately.
