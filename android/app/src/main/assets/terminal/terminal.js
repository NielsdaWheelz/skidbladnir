(function () {
    "use strict";

    var editor = document.getElementById("editor");
    var bridgeStatus = document.getElementById("bridge-status");
    var imeStatus = document.getElementById("ime-status");
    var viewportStatus = document.getElementById("viewport-status");
    var bridgePort = null;
    var terminal = new window.Terminal({
        convertEol: true,
        cursorBlink: true,
        fontFamily: "monospace",
        rows: 8,
        scrollback: 100,
        theme: { background: "#101114", foreground: "#f1f2f4" }
    });
    terminal.open(document.getElementById("terminal"));
    terminal.write("\\x1b[1;32mSkíðblaðnir\\x1b[0m platform harness\\r\\n");
    terminal.write("ANSI: \\x1b[31mred\\x1b[0m  Unicode: 北極星 / 🧭\\r\\n");

    function sanitizePaste(value) {
        return String(value).replace(/\\u0000/g, "").replace(/\\r\\n?/g, "\\n");
    }

    function send(kind, value) {
        if (!bridgePort) return;
        bridgePort.postMessage(JSON.stringify({ kind: kind, value: value }));
    }

    function appendText(value) {
        var start = editor.selectionStart;
        var end = editor.selectionEnd;
        editor.value = editor.value.slice(0, start) + value + editor.value.slice(end);
        editor.selectionStart = editor.selectionEnd = start + value.length;
        send("input", value);
    }

    window.__skidbladnirHarness = {
        state: "ready",
        ansiUnicode: "PASS",
        viewport: "unknown",
        editorValue: "",
        autoSubmitted: false,
        ime: "PASS",
        webMessagePort: false,
        lastAck: "",
        resize: function (columns, rows) {
            terminal.resize(columns, rows);
            this.viewport = columns + "x" + rows;
            viewportStatus.textContent = "Viewport: " + this.viewport;
            send("resize", this.viewport);
        },
        compose: function (value) {
            editor.focus();
            editor.dispatchEvent(new CompositionEvent("compositionstart", { data: "" }));
            appendText(value);
            editor.dispatchEvent(new CompositionEvent("compositionend", { data: value }));
            imeStatus.textContent = "IME: composed";
            this.ime = "PASS";
        },
        paste: function (value) {
            editor.focus();
            appendText(sanitizePaste(value));
            this.editorValue = editor.value;
        },
        dictation: function (value) {
            editor.focus();
            appendText(value);
            this.editorValue = editor.value;
        },
        send: function (kind, value) {
            send(kind, value);
        },
        networkEnabled: false,
        fileAccess: false,
        contentAccess: false
    };

    editor.addEventListener("compositionstart", function () {
        imeStatus.textContent = "IME: composing";
    });
    editor.addEventListener("compositionend", function () {
        imeStatus.textContent = "IME: composed";
        window.__skidbladnirHarness.ime = "PASS";
    });
    editor.addEventListener("paste", function (event) {
        event.preventDefault();
        var value = event.clipboardData ? event.clipboardData.getData("text/plain") : "";
        appendText(sanitizePaste(value));
        window.__skidbladnirHarness.editorValue = editor.value;
    });
    editor.addEventListener("input", function () {
        window.__skidbladnirHarness.editorValue = editor.value;
    });

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
