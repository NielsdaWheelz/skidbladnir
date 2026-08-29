(function () {
    "use strict";

    var terminalHost = document.getElementById("terminal");
    var terminalStatus = document.getElementById("terminal-status");
    var pagePort = null;
    var pageFailed = false;
    var fitScheduled = false;
    var fontsSettled = false;
    var modifiers = { control: "Off", alt: "Off" };
    var pendingProvenKey = null;
    var compositionActive = false;
    var maximumInputBytes = 1024 * 1024;
    var minimumColumns = 80;
    var minimumFontSize = 6;
    var maximumFontSize = 14;
    var lastPublishedColumns = 0;
    var lastPublishedRows = 0;
    var terminal = null;

    window.addEventListener("message", acceptPagePort);
    if (!terminalHost || !terminalStatus) {
        failPage();
        return;
    }

    // xterm reads extendedAnsi as one array anchored at ansi index 16, so the
    // 24-step grayscale (indices 232-255) lands at offsets 216-239 and the
    // 6x6x6 cube keeps its library defaults.
    var ink = [0x0c, 0x0d, 0x0f];
    var bone = [0xf3, 0xf0, 0xe8];
    var extendedAnsi = new Array(240);
    for (var step = 0; step < 24; step += 1) {
        extendedAnsi[216 + step] = "#" + ink.map(function (channel, index) {
            var tone = Math.round(channel + (bone[index] - channel) * step / 23);
            return (tone < 16 ? "0" : "") + tone.toString(16);
        }).join("");
    }

    terminal = new window.Terminal({
        cursorBlink: true,
        fontFamily: '"JetBrains Mono", monospace',
        fontSize: 14,
        minimumContrastRatio: 3,
        rows: 8,
        scrollback: 1000,
        screenReaderMode: true,
        theme: {
            background: "#0c0d0f",
            foreground: "#f3f0e8",
            cursor: "#d6a85f",
            cursorAccent: "#0c0d0f",
            selectionBackground: "#f3f0e84d",
            selectionInactiveBackground: "#f3f0e826",
            overviewRulerBorder: "#aaa69d",
            black: "#15171a",
            red: "#d74e33",
            green: "#4f925c",
            yellow: "#ac7e35",
            blue: "#538bac",
            magenta: "#bb5897",
            cyan: "#459c93",
            white: "#aaa69d",
            brightBlack: "#5c6370",
            brightRed: "#e46c55",
            brightGreen: "#76b082",
            brightYellow: "#d6a85f",
            brightBlue: "#78a9c6",
            brightMagenta: "#cd70ab",
            brightCyan: "#64c4ba",
            brightWhite: "#f3f0e8",
            extendedAnsi: extendedAnsi
        }
    });
    var fitAddon = new window.FitAddon.FitAddon();
    terminal.loadAddon(fitAddon);
    terminal.open(terminalHost);

    function send(payload) {
        if (pagePort) pagePort.postMessage(JSON.stringify(payload));
    }

    function clearProvenKey() {
        pendingProvenKey = null;
    }

    function failPage() {
        if (pageFailed) return;
        pageFailed = true;
        clearProvenKey();
        resetModifiers();
        send({ kind: "PageFailure" });
        pagePort = null;
    }

    function publishModifiers() {
        send({
            kind: "ModifierState",
            control: modifiers.control,
            alt: modifiers.alt
        });
    }

    function setModifiers(control, alt) {
        if (modifiers.control === control && modifiers.alt === alt) return;
        modifiers = { control: control, alt: alt };
        publishModifiers();
    }

    function resetModifiers() {
        setModifiers("Off", "Off");
    }

    function utf8ByteCount(value) {
        if (value.length > maximumInputBytes) return null;
        var count = 0;
        for (var index = 0; index < value.length; index += 1) {
            var first = value.charCodeAt(index);
            var code = first;
            if (first >= 0xd800 && first <= 0xdbff) {
                var second = value.charCodeAt(index + 1);
                if (second < 0xdc00 || second > 0xdfff) return null;
                code = value.codePointAt(index);
                index += 1;
            } else if (first >= 0xdc00 && first <= 0xdfff) {
                return null;
            }
            count += code <= 0x7f ? 1 : code <= 0x7ff ? 2 : code <= 0xffff ? 3 : 4;
            if (count > maximumInputBytes) return null;
        }
        return count;
    }

    function sendInput(value) {
        if (typeof value !== "string" || utf8ByteCount(value) === null) {
            failPage();
            return;
        }
        send({ kind: "Input", value: value });
    }

    function controlValue(value) {
        if (value.length !== 1) return value;
        var code = value.charCodeAt(0);
        if (code >= 0x61 && code <= 0x7a) code -= 0x20;
        if (code >= 0x40 && code <= 0x5f) return String.fromCharCode(code & 0x1f);
        if (code === 0x3f) return "\u007f";
        return value;
    }

    function acceptInput(value, keyProven) {
        var armed = modifiers;
        resetModifiers();
        if (keyProven && armed.control === "Armed") value = controlValue(value);
        if (keyProven && armed.alt === "Armed") value = "\u001b" + value;
        sendInput(value);
    }

    function pasteInput(value) {
        acceptInput(
            terminal.modes.bracketedPasteMode ? "\u001b[200~" + value + "\u001b[201~" : value,
            false
        );
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
        if (fontsSettled && pagePort && (columns !== lastPublishedColumns || rows !== lastPublishedRows)) {
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

    function exactObject(value, expectedKeys) {
        if (!value || typeof value !== "object" || Array.isArray(value)) return false;
        var actualKeys = Object.keys(value);
        if (actualKeys.length !== expectedKeys.length) return false;
        return expectedKeys.every(function (key) {
            return Object.prototype.hasOwnProperty.call(value, key);
        });
    }

    function parseObject(value) {
        if (typeof value !== "string") return null;
        try {
            var parsed = JSON.parse(value);
            return parsed && typeof parsed === "object" && !Array.isArray(parsed) ? parsed : null;
        } catch (error) {
            return null;
        }
    }

    function decodeBase64(value) {
        if (typeof value !== "string" || value.length % 4 !== 0 ||
            !/^(?:[A-Za-z0-9+/]{4})*(?:[A-Za-z0-9+/]{2}==|[A-Za-z0-9+/]{3}=)?$/.test(value)) {
            return null;
        }
        try {
            var decoded = window.atob(value);
            var bytes = new Uint8Array(decoded.length);
            for (var index = 0; index < decoded.length; index += 1) bytes[index] = decoded.charCodeAt(index);
            return bytes;
        } catch (error) {
            return null;
        }
    }

    function keyIsProven(event) {
        var domEvent = event && event.domEvent;
        if (!domEvent || domEvent.isTrusted !== true || compositionActive || domEvent.isComposing) return false;
        if (domEvent.type !== "keydown" && domEvent.type !== "keypress") return false;
        if (domEvent.keyCode === 229 || domEvent.which === 229) return false;
        if (domEvent.key === "Process" || domEvent.key === "Unidentified") return false;
        if (typeof domEvent.key !== "string" || typeof event.key !== "string") return false;
        if (domEvent.key !== event.key || event.key.length !== 1) return false;
        var code = event.key.charCodeAt(0);
        return code >= 0x20 && code <= 0x7e;
    }

    terminal.onKey(function (event) {
        clearProvenKey();
        if (!keyIsProven(event)) return;
        var candidate = { value: event.key };
        pendingProvenKey = candidate;
        Promise.resolve().then(function () {
            if (pendingProvenKey === candidate) clearProvenKey();
        });
    });

    terminal.onData(function (value) {
        var keyProven = pendingProvenKey !== null && pendingProvenKey.value === value;
        clearProvenKey();
        acceptInput(value, keyProven);
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

    if (!input || !composition || !screen) {
        failPage();
        return;
    }
    ["compositionstart", "compositionend", "beforeinput", "input", "keydown", "focus"]
        .forEach(function (eventName) {
            input.addEventListener(eventName, scheduleImeContainment);
        });
    input.addEventListener("beforeinput", clearProvenKey, true);
    input.addEventListener("compositionstart", function () {
        compositionActive = true;
        clearProvenKey();
    }, true);
    input.addEventListener("compositionend", function () {
        clearProvenKey();
        compositionActive = false;
    }, true);
    input.addEventListener("compositionupdate", function (event) {
        if (typeof event.data === "string") {
            composition.textContent = "\u200e" + event.data + "\u200e";
        }
        scheduleImeContainment();
    });
    input.addEventListener("paste", function (event) {
        clearProvenKey();
        event.preventDefault();
        event.stopImmediatePropagation();
        var sanitized = sanitizePaste(event.clipboardData ? event.clipboardData.getData("text/plain") : "");
        if (sanitized === null) {
            failPage();
        } else {
            pasteInput(sanitized);
        }
    }, true);

    new ResizeObserver(scheduleFit).observe(terminalHost);
    window.addEventListener("resize", scheduleFit);
    window.addEventListener("orientationchange", scheduleFit);
    window.addEventListener("scroll", lockPageViewport);
    if (window.visualViewport) {
        window.visualViewport.addEventListener("resize", scheduleFit);
        window.visualViewport.addEventListener("scroll", lockPageViewport);
    }

    function settleFonts() {
        fontsSettled = true;
        // xterm caches the cell size measured at open(), before the vendored
        // face can arrive, and re-measures only on a font option change.
        terminal._core._charSizeService.measure();
        scheduleFit();
    }

    // Geometry stays local until the vendored faces settle, so the 80-column
    // guarantee is measured in the real font; a rejected load degrades to
    // monospace rather than withholding geometry.
    Promise.all([
        document.fonts.load('14px "JetBrains Mono"'),
        document.fonts.load('bold 14px "JetBrains Mono"')
    ]).then(settleFonts, settleFonts);

    function accessoryInput(key) {
        var literals = {
            Escape: "\u001b",
            Slash: "/",
            Hyphen: "-",
            Tab: "\t"
        };
        if (Object.prototype.hasOwnProperty.call(literals, key)) {
            return { value: literals[key], modifiersEligible: true };
        }
        var suffixes = {
            Left: "D",
            Up: "A",
            Down: "B",
            Right: "C",
            Home: "H",
            End: "F"
        };
        var pageParameters = { PageUp: "5", PageDown: "6" };
        var controlArmed = modifiers.control === "Armed";
        var altArmed = modifiers.alt === "Armed";
        var modified = controlArmed || altArmed;
        var parameter = 1 + (altArmed ? 2 : 0) + (controlArmed ? 4 : 0);
        if (Object.prototype.hasOwnProperty.call(suffixes, key)) {
            if (modified) {
                return { value: "\u001b[1;" + parameter + suffixes[key], modifiersEligible: false };
            }
            return {
                value: "\u001b" + (terminal.modes.applicationCursorKeysMode ? "O" : "[") + suffixes[key],
                modifiersEligible: false
            };
        }
        if (Object.prototype.hasOwnProperty.call(pageParameters, key)) {
            var page = pageParameters[key];
            return {
                value: "\u001b[" + page + (modified ? ";" + parameter : "") + "~",
                modifiersEligible: false
            };
        }
        return null;
    }

    function acceptAccessory(key) {
        clearProvenKey();
        if (key === "Control") {
            setModifiers(modifiers.control === "Off" ? "Armed" : "Off", modifiers.alt);
            focusTerminal();
            return;
        }
        if (key === "Alt") {
            setModifiers(modifiers.control, modifiers.alt === "Off" ? "Armed" : "Off");
            focusTerminal();
            return;
        }
        var input = accessoryInput(key);
        if (input === null) {
            failPage();
            return;
        }
        acceptInput(input.value, input.modifiersEligible);
        focusTerminal();
    }

    function acceptNativeMessage(message) {
        var payload = parseObject(message.data);
        if (!payload) {
            failPage();
            return;
        }
        if (payload.kind === "Output" && exactObject(payload, ["kind", "sequence", "data"]) &&
            typeof payload.sequence === "string") {
            var bytes = decodeBase64(payload.data);
            if (bytes === null) {
                failPage();
                return;
            }
            terminal.write(bytes, function () {
                terminalStatus.textContent = "Terminal connected";
                send({ kind: "OutputApplied", sequence: payload.sequence });
            });
            return;
        }
        if (payload.kind === "Focus" && exactObject(payload, ["kind"])) {
            focusTerminal();
            return;
        }
        if (payload.kind === "Accessory" && exactObject(payload, ["kind", "key"]) &&
            typeof payload.key === "string") {
            acceptAccessory(payload.key);
            return;
        }
        if (payload.kind === "ResetModifiers" && exactObject(payload, ["kind"])) {
            clearProvenKey();
            resetModifiers();
            return;
        }
        failPage();
    }

    function acceptPagePort(event) {
        var handshake = parseObject(event.data);
        var validHandshake = event.ports && event.ports.length === 1 &&
            handshake && exactObject(handshake, ["kind", "version"]) &&
            handshake.kind === "PagePort" && handshake.version === 1;
        if (!validHandshake) {
            failPage();
            return;
        }
        if (pagePort !== null) {
            failPage();
            return;
        }
        pagePort = event.ports[0];
        if (pageFailed) {
            send({ kind: "PageFailure" });
            pagePort = null;
            return;
        }
        pagePort.onmessage = acceptNativeMessage;
        pagePort.onmessageerror = failPage;
        pagePort.start();
        publishModifiers();
        send({ kind: "Ready" });
        scheduleFit();
    }
}());
