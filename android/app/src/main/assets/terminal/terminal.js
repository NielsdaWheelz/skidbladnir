(function () {
    "use strict";

    var bridgeStatus = document.getElementById("bridge-status");
    var focusTerminal = document.getElementById("focus-terminal");
    var inputStatus = document.getElementById("input-status");
    var imeStatus = document.getElementById("ime-status");
    var viewportStatus = document.getElementById("viewport-status");
    var draftValue = document.getElementById("draft-value");
    var draftStatus = document.getElementById("draft-status");
    var bridgePort = null;
    var inputHistory = [];
    var automaticReplies = [];
    var compositionValue = "";
    var compositionActive = false;
    var compositionCommitObserved = false;
    var compositionStartOffset = 0;
    var pendingCompositionValues = [];
    var visualViewportState = { width: 0, height: 0 };
    var terminalContainer = document.getElementById("terminal");
    var terminal = new window.Terminal({
        convertEol: true,
        cursorBlink: true,
        fontFamily: "monospace",
        rows: 8,
        scrollback: 100,
        ignoreBracketedPasteMode: true,
        screenReaderMode: true,
        theme: { background: "#101114", foreground: "#f1f2f4" }
    });
    var fitAddon = new window.FitAddon.FitAddon();
    var fitScheduled = false;
    terminal.loadAddon(fitAddon);
    terminal.open(terminalContainer);
    terminal.write("\x1b[1;32mSkíðblaðnir\x1b[0m platform harness\r\n");
    terminal.write("ANSI: \x1b[31mred\x1b[0m  Unicode: 北極星 / 🧭\r\n");

    function sanitizePaste(value) {
        var normalized = String(value).replace(/\r\n?/g, "\n");
        return Array.from(normalized).filter(function (character) {
            var code = character.charCodeAt(0);
            return code === 0x09 || code === 0x0a || (code > 0x1f && code < 0x7f) || code > 0x9f;
        }).join("");
    }

    function send(kind, value) {
        if (!bridgePort) return;
        bridgePort.postMessage(JSON.stringify({ kind: kind, value: value }));
    }

    function inputElement() {
        return document.querySelector(".xterm-helper-textarea");
    }

    function automaticReplyKind(data) {
        if (/^\x1b\[\?[^c]*c$/.test(data)) return "DA1";
        if (/^\x1b\[>[^c]*c$/.test(data)) return "DA2";
        if (/^\x1b\[[?]?\d+n$/.test(data)) return "DSR";
        if (/^\x1b\[[?]?\d+;\d+R$/.test(data)) return "CPR";
        return "";
    }

    function updateDraftValue() {
        var value = window.__skidbladnirHarness.editorValue +
            pendingCompositionValues.join("") + compositionValue;
        draftValue.textContent = value || "Empty draft";
        draftValue.setAttribute("aria-label", "Current editable draft: " + (value || "empty"));
        draftStatus.textContent = compositionActive || pendingCompositionValues.length > 0
            ? "Draft: composing; never submitted or saved"
            : "Draft: editable; never submitted or saved";
    }

    function updateVisualViewport() {
        var source = window.visualViewport;
        var width = source ? source.width : window.innerWidth;
        var height = source ? source.height : window.innerHeight;
        visualViewportState.width = Math.round(width || 0);
        visualViewportState.height = Math.round(height || 0);
        if (window.__skidbladnirHarness) {
            window.__skidbladnirHarness.visualViewport = visualViewportState;
        }
        viewportStatus.textContent = "Terminal: " + window.__skidbladnirHarness.viewport +
            " | Visual viewport: " + visualViewportState.width + "x" + visualViewportState.height + " px";
    }

    function fitTerminal() {
        fitScheduled = false;
        fitAddon.fit();
        if (!window.__skidbladnirHarness) return;
        window.__skidbladnirHarness.viewport = terminal.cols + "x" + terminal.rows;
        updateVisualViewport();
        send("resize", window.__skidbladnirHarness.viewport);
    }

    function scheduleTerminalFit() {
        if (fitScheduled) return;
        fitScheduled = true;
        window.requestAnimationFrame(fitTerminal);
    }

    function updateDraftFromInput(value) {
        Array.from(value).forEach(function (character) {
            if (character === "\x7f" || character === "\b") {
                window.__skidbladnirHarness.editorValue = Array.from(
                    window.__skidbladnirHarness.editorValue,
                ).slice(0, -1).join("");
            } else {
                window.__skidbladnirHarness.editorValue += character;
            }
        });
        updateDraftValue();
    }

    function recordInput(data) {
        var reply = automaticReplyKind(data);
        if (reply) {
            automaticReplies.push(reply);
            send("terminalReply", reply);
            return;
        }
        var matchesActiveComposition = compositionActive && data === compositionValue;
        var matchesPendingComposition = pendingCompositionValues.length > 0 &&
            data === pendingCompositionValues[0];
        if (matchesActiveComposition && !matchesPendingComposition) {
            compositionCommitObserved = true;
            compositionValue = "";
        } else if (pendingCompositionValues.length > 0) {
            pendingCompositionValues.shift();
        } else if (compositionActive) {
            compositionCommitObserved = true;
            compositionValue = "";
        }
        var normalized = data.replace(/\r\n?/g, "\n");
        inputHistory.push(normalized);
        updateDraftFromInput(normalized);
        send("input", normalized);
        inputStatus.textContent = "Input: received through xterm";
    }

    terminal.onData(function (data) {
        recordInput(data);
    });

    var terminalInput = inputElement();
    if (terminalInput) {
        terminalInput.addEventListener("compositionstart", function (event) {
            compositionActive = true;
            compositionCommitObserved = false;
            compositionStartOffset = terminalInput.value.length;
            compositionValue = event.data || "";
            updateDraftValue();
        });
        terminalInput.addEventListener("compositionupdate", function (event) {
            compositionValue = event.data || "";
            updateDraftValue();
        });
        terminalInput.addEventListener("beforeinput", function (event) {
            if (!event.isComposing || !event.data) return;
            compositionActive = true;
            compositionValue = event.data;
            updateDraftValue();
        }, true);
        terminalInput.addEventListener("input", function (event) {
            if (!event.isComposing || (!event.data && !terminalInput.value)) return;
            compositionActive = true;
            var currentValue = terminalInput.value.length >= compositionStartOffset
                ? terminalInput.value.substring(compositionStartOffset)
                : "";
            compositionValue = currentValue || event.data || compositionValue;
            updateDraftValue();
        }, true);
        terminalInput.addEventListener("compositionend", function (event) {
            compositionActive = false;
            if (compositionCommitObserved) {
                compositionCommitObserved = false;
                compositionValue = "";
                updateDraftValue();
                return;
            }
            compositionValue = event.data || "";
            if (compositionValue !== "") pendingCompositionValues.push(compositionValue);
            compositionValue = "";
            updateDraftValue();
        });
    }

    focusTerminal.addEventListener("click", function () {
        terminal.focus();
        inputStatus.textContent = "Input: terminal focused; type with Gboard";
    });

    window.__skidbladnirHarness = {
        state: "ready",
        ansiUnicode: "PASS",
        viewport: "unknown",
        editorValue: "",
        inputHistory: inputHistory,
        automaticReplies: automaticReplies,
        autoSubmitted: false,
        ime: "PASS",
        actualInputElement: inputElement() !== null,
        screenReaderMode: true,
        webMessagePort: false,
        lastAck: "",
        visualViewport: visualViewportState,
        resize: function (columns, rows) {
            terminal.resize(columns, rows);
            this.viewport = columns + "x" + rows;
            updateVisualViewport();
            send("resize", this.viewport);
        },
        backspace: function () {
            terminal.focus();
            terminal.paste("\x7f");
        },
        compose: function (value) {
            var input = inputElement();
            terminal.focus();
            if (input) input.dispatchEvent(new CompositionEvent("compositionstart", { data: "" }));
            if (input) input.dispatchEvent(new CompositionEvent("compositionupdate", { data: String(value) }));
            if (input) input.dispatchEvent(new CompositionEvent("compositionend", { data: String(value) }));
            terminal.paste(String(value));
            imeStatus.textContent = "IME: composed";
            this.ime = "PASS";
        },
        paste: function (value) {
            var sanitized = sanitizePaste(value);
            terminal.focus();
            terminal.paste(sanitized);
        },
        dictation: function (value) {
            terminal.focus();
            terminal.paste(String(value));
        },
        probeAutomaticReplies: function () {
            automaticReplies.length = 0;
            terminal.write("\x1b[c\x1b[>c\x1b[5n\x1b[6n");
            return "requested";
        },
        send: function (kind, value) {
            send(kind, value);
        },
        networkEnabled: false,
        fileAccess: false,
        contentAccess: false
    };

    updateDraftValue();
    scheduleTerminalFit();
    new ResizeObserver(scheduleTerminalFit).observe(terminalContainer);
    window.addEventListener("resize", scheduleTerminalFit);
    window.addEventListener("orientationchange", scheduleTerminalFit);
    if (window.visualViewport) window.visualViewport.addEventListener("resize", scheduleTerminalFit);

    window.addEventListener("message", function (event) {
        if (!event.ports || !event.ports.length) return;
        bridgePort = event.ports[0];
        bridgePort.onmessage = function (message) {
            var payload = JSON.parse(message.data);
            if (payload.kind === "ack") {
                window.__skidbladnirHarness.lastAck = payload.for;
                bridgeStatus.textContent = "Native WebMessagePort connected";
            }
        };
        bridgePort.start();
        window.__skidbladnirHarness.webMessagePort = true;
        bridgeStatus.textContent = "Native WebMessagePort connected";
        send("ready", "terminal-harness");
    });
}());
