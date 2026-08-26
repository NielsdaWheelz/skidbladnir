(function () {
    "use strict";

    var terminalHost = document.getElementById("terminal");
    var terminalStatus = document.getElementById("terminal-status");
    var pagePort = null;
    var pageFailed = false;
    var fitScheduled = false;
    var maximumInputBytes = 1024 * 1024;
    var minimumColumns = 80;
    var minimumFontSize = 6;
    var maximumFontSize = 14;
    var lastPublishedColumns = 0;
    var lastPublishedRows = 0;
    var terminal = new window.Terminal({
        cursorBlink: true,
        fontFamily: "monospace",
        fontSize: 14,
        rows: 8,
        scrollback: 1000,
        screenReaderMode: true,
        theme: {
            background: "#0c0d0f",
            foreground: "#f3f0e8",
            cursor: "#d6a85f",
            selectionBackground: "#725b36",
            black: "#202328",
            red: "#e06c75",
            green: "#98c379",
            yellow: "#e5c07b",
            blue: "#61afef",
            magenta: "#c678dd",
            cyan: "#56b6c2",
            white: "#d7dae0",
            brightBlack: "#5c6370",
            brightRed: "#ef7b86",
            brightGreen: "#a9d18e",
            brightYellow: "#f0cf88",
            brightBlue: "#75bdf4",
            brightMagenta: "#d38be8",
            brightCyan: "#6bc4cf",
            brightWhite: "#ffffff"
        }
    });
    var fitAddon = new window.FitAddon.FitAddon();
    terminal.loadAddon(fitAddon);
    terminal.open(terminalHost);

    function send(payload) {
        if (pagePort) pagePort.postMessage(JSON.stringify(payload));
    }

    function failPage() {
        if (pageFailed) return;
        pageFailed = true;
        send({ kind: "PageFailure" });
        pagePort = null;
    }

    function utf8ByteCount(value) {
        if (value.length > maximumInputBytes) return null;
        var count = 0;
        for (var index = 0; index < value.length; index += 1) {
            var code = value.codePointAt(index);
            if (code > 0xffff) index += 1;
            count += code <= 0x7f ? 1 : code <= 0x7ff ? 2 : code <= 0xffff ? 3 : 4;
            if (count > maximumInputBytes) return null;
        }
        return count;
    }

    function sendInput(value) {
        if (utf8ByteCount(value) === null) {
            failPage();
            return;
        }
        send({ kind: "Input", value: value });
    }

    function pasteInput(value) {
        sendInput(terminal.modes.bracketedPasteMode ? "\u001b[200~" + value + "\u001b[201~" : value);
    }

    function sanitizePaste(value) {
        value = String(value);
        if (value.length > maximumInputBytes) return null;
        var sanitized = "";
        var byteCount = 0;
        for (var index = 0; index < value.length; index += 1) {
            var code = value.codePointAt(index);
            var character = String.fromCodePoint(code);
            if (code > 0xffff) index += 1;
            if (code === 0x0d) {
                if (value.charCodeAt(index + 1) === 0x0a) index += 1;
                code = 0x0a;
                character = "\n";
            }
            if (code >= 0xd800 && code <= 0xdfff) continue;
            if (code === 0x09 || code === 0x0a || (code > 0x1f && code < 0x7f) || code > 0x9f) {
                byteCount += code <= 0x7f ? 1 : code <= 0x7ff ? 2 : code <= 0xffff ? 3 : 4;
                if (byteCount > maximumInputBytes) return null;
                sanitized += character;
            }
        }
        return sanitized;
    }

    function resizeTerminal() {
        fitScheduled = false;
        if (terminalHost.clientWidth === 0 || terminalHost.clientHeight === 0) return;
        var dimensions = fitAddon.proposeDimensions();
        if (!dimensions) return;
        var currentFontSize = terminal.options.fontSize;
        var targetFontSize = Math.max(
            minimumFontSize,
            Math.min(maximumFontSize, currentFontSize * dimensions.cols / minimumColumns)
        );
        targetFontSize = Math.floor(targetFontSize * 100) / 100;
        if (dimensions.cols < minimumColumns && targetFontSize >= currentFontSize - 0.01) {
            targetFontSize = Math.max(minimumFontSize, currentFontSize - 0.1);
        }
        if (Math.abs(targetFontSize - currentFontSize) >= 0.05) {
            terminal.options.fontSize = targetFontSize;
            scheduleFit();
            return;
        }
        var columns = Math.max(20, Math.min(240, dimensions.cols));
        var rows = Math.max(5, Math.min(120, dimensions.rows));
        if (terminal.cols !== columns || terminal.rows !== rows) terminal.resize(columns, rows);
        if (pagePort && (columns !== lastPublishedColumns || rows !== lastPublishedRows)) {
            lastPublishedColumns = columns;
            lastPublishedRows = rows;
            send({ kind: "Resize", columns: columns, rows: rows });
        }
    }

    function scheduleFit() {
        if (fitScheduled) return;
        fitScheduled = true;
        window.requestAnimationFrame(resizeTerminal);
    }

    function decodeBase64(value) {
        var decoded = window.atob(value);
        var bytes = new Uint8Array(decoded.length);
        for (var index = 0; index < decoded.length; index += 1) bytes[index] = decoded.charCodeAt(index);
        return bytes;
    }

    terminal.onData(function (value) {
        sendInput(value);
    });

    var input = document.querySelector(".xterm-helper-textarea");
    var composition = document.querySelector(".composition-view");
    var screen = document.querySelector(".xterm-screen");

    function lockPageViewport() {
        document.documentElement.scrollLeft = 0;
        document.documentElement.scrollTop = 0;
        document.body.scrollLeft = 0;
        document.body.scrollTop = 0;
        window.scrollTo(0, 0);
    }

    function containImeGeometry() {
        if (!input || !composition || !screen) {
            failPage();
            return;
        }
        var screenBounds = screen.getBoundingClientRect();
        var compositionBounds = composition.getBoundingClientRect();
        var inputBounds = input.getBoundingClientRect();
        var activeInputLeft = Math.max(compositionBounds.left, inputBounds.left);
        var boundedLeft = Math.max(screenBounds.left, Math.min(activeInputLeft, screenBounds.right - 1));
        var maximumWidth = Math.max(screenBounds.right - boundedLeft, 1);
        composition.style.maxWidth = maximumWidth + "px";
        composition.style.overflow = "hidden";
        composition.style.direction = "rtl";
        input.style.width = Math.min(Math.max(inputBounds.width, 1), maximumWidth) + "px";
        input.style.maxWidth = maximumWidth + "px";
        input.style.overflow = "hidden";
        lockPageViewport();
    }

    function scheduleImeContainment() {
        containImeGeometry();
        window.setTimeout(containImeGeometry, 0);
        window.requestAnimationFrame(containImeGeometry);
    }

    function focusTerminal() {
        if (!input) {
            failPage();
            return;
        }
        input.focus({ preventScroll: true });
        scheduleImeContainment();
    }

    if (input) {
        ["compositionstart", "compositionend", "beforeinput", "input", "keydown", "focus"]
            .forEach(function (eventName) {
                input.addEventListener(eventName, scheduleImeContainment);
            });
        input.addEventListener("compositionupdate", function (event) {
            if (composition && typeof event.data === "string") {
                composition.textContent = "\u200e" + event.data + "\u200e";
            }
            scheduleImeContainment();
        });
        input.addEventListener("paste", function (event) {
            event.preventDefault();
            event.stopImmediatePropagation();
            var sanitized = sanitizePaste(event.clipboardData ? event.clipboardData.getData("text/plain") : "");
            if (sanitized === null) {
                failPage();
            } else {
                pasteInput(sanitized);
            }
        }, true);
    } else {
        failPage();
    }

    new ResizeObserver(scheduleFit).observe(terminalHost);
    window.addEventListener("resize", scheduleFit);
    window.addEventListener("orientationchange", scheduleFit);
    window.addEventListener("scroll", lockPageViewport);
    if (window.visualViewport) {
        window.visualViewport.addEventListener("resize", scheduleFit);
        window.visualViewport.addEventListener("scroll", lockPageViewport);
    }

    window.addEventListener("message", function (event) {
        if (!event.ports || event.ports.length !== 1) return;
        var handshake = null;
        try {
            handshake = JSON.parse(event.data);
        } catch (error) {
            handshake = null;
        }
        if (!handshake || handshake.kind !== "PagePort" || handshake.version !== 1) {
            failPage();
            return;
        }
        pagePort = event.ports[0];
        if (pageFailed) {
            send({ kind: "PageFailure" });
            pagePort = null;
            return;
        }
        pagePort.onmessage = function (message) {
            var payload = JSON.parse(message.data);
            if (payload.kind === "Output") {
                terminal.write(decodeBase64(payload.data), function () {
                    terminalStatus.textContent = "Terminal connected";
                    send({ kind: "OutputApplied", sequence: payload.sequence });
                });
            } else if (payload.kind === "Focus") {
                focusTerminal();
            } else if (payload.kind === "Accessory") {
                var literal = {
                    Escape: "\u001b",
                    CtrlC: "\u0003",
                    Tab: "\t",
                    Newline: "\n"
                }[payload.key];
                var suffix = {
                    Left: "D",
                    Up: "A",
                    Down: "B",
                    Right: "C",
                    Home: "H",
                    End: "F"
                }[payload.key];
                if (!literal && !suffix) {
                    failPage();
                    return;
                }
                sendInput(literal || "\u001b" + (terminal.modes.applicationCursorKeysMode ? "O" : "[") + suffix);
                focusTerminal();
            }
        };
        pagePort.start();
        send({ kind: "Ready" });
        scheduleFit();
    });
}());
