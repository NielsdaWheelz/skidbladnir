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
    var maximumSelectionBytes = 256 * 1024;
    // The gateway's Resize bounds; the page fits down to them and never up.
    var minimumColumns = 20;
    var maximumColumns = 240;
    var minimumRows = 5;
    var maximumRows = 120;
    var lastPublishedColumns = 0;
    var lastPublishedRows = 0;
    var viewportTooSmallPublished = false;
    var terminal = null;
    var terminalTouchInteraction = null;

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
        if (terminalTouchInteraction) terminalTouchInteraction.dispose();
        clearProvenKey();
        setModifiers("Off", "Off");
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

    function resetInputState() {
        clearProvenKey();
        if (terminalTouchInteraction) terminalTouchInteraction.cancel();
        setModifiers("Off", "Off");
    }

    function utf8ByteCount(value, maximumBytes) {
        if (value.length > maximumBytes) return null;
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
            if (count > maximumBytes) return null;
        }
        return count;
    }

    function isUnicodeScalarSequence(value) {
        for (var index = 0; index < value.length; index += 1) {
            var first = value.charCodeAt(index);
            if (first >= 0xd800 && first <= 0xdbff) {
                var second = value.charCodeAt(index + 1);
                if (second < 0xdc00 || second > 0xdfff) return false;
                index += 1;
            } else if (first >= 0xdc00 && first <= 0xdfff) {
                return false;
            }
        }
        return true;
    }

    function sendInput(value) {
        if (typeof value !== "string" || utf8ByteCount(value, maximumInputBytes) === null) {
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
        setModifiers("Off", "Off");
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

    // Whole cells at the native-chosen font, capped downward at the gateway
    // maximum and never clamped upward. Below the gateway minimum xterm keeps
    // its last valid grid and native hears ViewportTooSmall once per episode;
    // recovery republishes Resize even when the grid is unchanged.
    function resizeTerminal() {
        fitScheduled = false;
        var dimensions = terminalHost.clientWidth > 0 && terminalHost.clientHeight > 0 ?
            fitAddon.proposeDimensions() : undefined;
        if (!dimensions || dimensions.cols < minimumColumns || dimensions.rows < minimumRows) {
            if (fontsSettled && pagePort && !viewportTooSmallPublished) {
                viewportTooSmallPublished = true;
                send({ kind: "ViewportTooSmall" });
            }
            return;
        }
        var columns = Math.min(maximumColumns, dimensions.cols);
        var rows = Math.min(maximumRows, dimensions.rows);
        if (terminal.cols !== columns || terminal.rows !== rows) terminal.resize(columns, rows);
        if (!fontsSettled || !pagePort) return;
        if (viewportTooSmallPublished || columns !== lastPublishedColumns || rows !== lastPublishedRows) {
            viewportTooSmallPublished = false;
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

    function isFontSizeCssPx(value) {
        return typeof value === "number" && Number.isFinite(value) && value > 0;
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

    function createTerminalTouchInteraction(options) {
        var ownerTerminal = options.terminal;
        var ownerScreen = options.screen;
        var longPressMilliseconds = options.longPressMilliseconds;
        var ownerInput = document.querySelector(".xterm-helper-textarea");
        var touchSlop = 8;
        var gesture = null;
        var selectionGeneration = null;
        var lastAcknowledgedSelectionGeneration = null;
        var nextSelectionGeneration = 1;
        var disposed = false;

        function canonicalSelectionGeneration(value) {
            if (typeof value !== "string" || !/^[1-9][0-9]*$/.test(value)) return null;
            var numeric = Number(value);
            return Number.isSafeInteger(numeric) && numeric > 0 && String(numeric) === value ?
                value : null;
        }

        function compareSelectionGenerations(left, right) {
            if (left.length !== right.length) return left.length < right.length ? -1 : 1;
            return left === right ? 0 : left < right ? -1 : 1;
        }

        function beginSelectionGeneration() {
            if (selectionGeneration !== null && !selectionGeneration.clearAcknowledged) {
                failPage();
                return null;
            }
            if (!Number.isSafeInteger(nextSelectionGeneration) || nextSelectionGeneration <= 0) {
                failPage();
                return null;
            }
            var generation = String(nextSelectionGeneration);
            nextSelectionGeneration += 1;
            selectionGeneration = { value: generation, clearAcknowledged: false };
            return generation;
        }

        function isTrustedTouch(event) {
            return event.isTrusted === true;
        }

        function isScreenTouch(event) {
            var target = event.target;
            return target === ownerScreen ||
                (target && target.nodeType === Node.ELEMENT_NODE && ownerScreen.contains(target));
        }

        function containIgnoredTouch(event) {
            if (event.cancelable) event.preventDefault();
            event.stopImmediatePropagation();
        }

        function consumeTouch(event, requireCancelable) {
            if (requireCancelable && !event.cancelable) {
                event.stopImmediatePropagation();
                failPage();
                return false;
            }
            if (event.cancelable) {
                event.preventDefault();
                if (!event.defaultPrevented) {
                    event.stopImmediatePropagation();
                    failPage();
                    return false;
                }
            }
            event.stopImmediatePropagation();
            return true;
        }

        function clearFrame(active) {
            if (active.frame === null || active.frame === undefined) return;
            window.cancelAnimationFrame(active.frame);
            active.frame = null;
        }

        function clearTimer(active) {
            if (active.timer === null || active.timer === undefined) return;
            window.clearTimeout(active.timer);
            active.timer = null;
        }

        function findTouch(list, identifier) {
            var found = null;
            for (var index = 0; index < list.length; index += 1) {
                if (list[index].identifier !== identifier) continue;
                if (found !== null) return null;
                found = list[index];
            }
            return found;
        }

        function acknowledgeSelectionCleared() {
            if (selectionGeneration === null) {
                failPage();
                return false;
            }
            if (!selectionGeneration.clearAcknowledged) {
                selectionGeneration.clearAcknowledged = true;
                lastAcknowledgedSelectionGeneration = selectionGeneration.value;
                send({
                    kind: "SelectionCleared",
                    generation: selectionGeneration.value
                });
            }
            return true;
        }

        function cancelGesture(blockTail) {
            var active = gesture;
            if (active === null) return;
            clearTimer(active);
            clearFrame(active);
            var clearsSemanticDrag = active.state === "Selecting";
            var clearsReleasedSelection = active.state === "Selected" ||
                active.state === "PendingSelected" || active.state === "BlockingSelected";
            if (active.state === "Scrolling") {
                active.accumulator = 0;
                active.emittedRows = 0;
            }
            gesture = blockTail && active.identifier !== undefined ?
                { state: "Blocked", identifier: active.identifier, frame: null, timer: null } : null;
            try {
                if (clearsSemanticDrag) {
                    ownerTerminal.handleSelectionInput({ kind: "Cancel" });
                } else if (clearsReleasedSelection) {
                    ownerTerminal.clearSelection();
                }
            } catch (error) {
                failPage();
                return;
            }
            if ((clearsSemanticDrag || clearsReleasedSelection) && !acknowledgeSelectionCleared()) return;
        }

        function clearSelectionFromNative() {
            var generation = arguments[0];
            if (canonicalSelectionGeneration(generation) === null) {
                failPage();
                return;
            }
            if (selectionGeneration === null || generation !== selectionGeneration.value) {
                if (lastAcknowledgedSelectionGeneration !== null &&
                    compareSelectionGenerations(generation, lastAcknowledgedSelectionGeneration) <= 0) return;
                failPage();
                return;
            }
            if (selectionGeneration.clearAcknowledged) return;
            var active = gesture;
            if (active !== null) {
                cancelGesture(true);
                if (pageFailed) return;
                if (active.state === "Selecting" || active.state === "Selected" ||
                    active.state === "PendingSelected" || active.state === "BlockingSelected") return;
            }
            try {
                ownerTerminal.clearSelection();
            } catch (error) {
                failPage();
                return;
            }
            acknowledgeSelectionCleared();
        }

        function sendWheelLines(deltaLines, clientX, clientY) {
            ownerTerminal.handleWheelInput({
                deltaLines: deltaLines,
                clientX: clientX,
                clientY: clientY,
                altKey: false,
                ctrlKey: false,
                shiftKey: false
            });
        }

        function wholeRows(displacement, rowHeight) {
            return displacement < 0 ?
                Math.ceil(displacement / rowHeight) :
                Math.floor(displacement / rowHeight);
        }

        function dispatchAccumulatedRows(active) {
            if (gesture !== active || active.state !== "Scrolling") return;
            if (ownerTerminal.hasSelection()) {
                cancelGesture(true);
                return;
            }
            var rows = wholeRows(active.accumulator, active.rowHeight);
            if (rows === 0) return;
            var rowLimit = ownerTerminal.rows;
            if (!Number.isFinite(rowLimit) || rowLimit <= 0) {
                cancelGesture(true);
                return;
            }
            var boundedRows = Math.max(-rowLimit, Math.min(rowLimit, rows));
            active.accumulator -= rows * active.rowHeight;
            active.emittedRows += boundedRows;
            sendWheelLines(boundedRows, active.clientX, active.clientY);
        }

        function scheduleDispatch(active) {
            if (active.frame !== null) return;
            active.frame = window.requestAnimationFrame(function () {
                active.frame = null;
                dispatchAccumulatedRows(active);
            });
        }

        function addMovement(active, currentY) {
            if (!Number.isFinite(currentY)) {
                cancelGesture(true);
                return false;
            }
            active.accumulator += active.previousY - currentY;
            active.previousY = currentY;
            return true;
        }

        function startSelection(active) {
            clearTimer(active);
            if (active.state === "PendingSelected") {
                try {
                    ownerTerminal.clearSelection();
                } catch (error) {
                    gesture = null;
                    failPage();
                    return;
                }
                if (!acknowledgeSelectionCleared()) return;
            }
            var generation = beginSelectionGeneration();
            if (generation === null) return;
            try {
                ownerTerminal.handleSelectionInput({
                    kind: "StartWord",
                    clientX: active.startX,
                    clientY: active.startY
                });
            } catch (error) {
                gesture = null;
                failPage();
                return;
            }
            active.state = "Selecting";
            send({ kind: "SelectionStarted", generation: generation });
        }

        function extendSelection(active, kind, clientX, clientY) {
            try {
                ownerTerminal.handleSelectionInput({
                    kind: kind,
                    clientX: clientX,
                    clientY: clientY
                });
                return true;
            } catch (error) {
                failPage();
                return false;
            }
        }

        function releaseSelection(active, touch) {
            if (!extendSelection(active, "End", touch.clientX, touch.clientY)) return;
            gesture = null;
            var text = ownerTerminal.getSelection();
            if (typeof text !== "string" || !isUnicodeScalarSequence(text)) {
                ownerTerminal.clearSelection();
                failPage();
                return;
            }
            if (text.length === 0) {
                ownerTerminal.clearSelection();
                acknowledgeSelectionCleared();
                return;
            }
            if (utf8ByteCount(text, maximumSelectionBytes) === null) {
                ownerTerminal.clearSelection();
                send({
                    kind: "SelectionCopyRejected",
                    generation: selectionGeneration.value,
                    reason: "TooLarge"
                });
                return;
            }
            var width = window.innerWidth;
            var height = window.innerHeight;
            if (!Number.isFinite(width) || width <= 0 || !Number.isFinite(height) || height <= 0) {
                ownerTerminal.clearSelection();
                failPage();
                return;
            }
            gesture = { state: "Selected", frame: null, timer: null };
            send({
                kind: "SelectionAvailable",
                generation: selectionGeneration.value,
                anchorX: Math.max(0, Math.min(1, touch.clientX / width)),
                anchorY: Math.max(0, Math.min(1, touch.clientY / height)),
                text: text
            });
        }

        function touchIsInsideScreen(touch) {
            var bounds = ownerScreen.getBoundingClientRect();
            return Number.isFinite(touch.clientX) && Number.isFinite(touch.clientY) &&
                touch.clientX > bounds.left && touch.clientX < bounds.right &&
                touch.clientY > bounds.top && touch.clientY < bounds.bottom;
        }

        function blockGesture(event) {
            cancelGesture(true);
            if (gesture === null) {
                gesture = { state: "Blocked", frame: null, timer: null };
            }
            if (event.touches.length === 0) gesture = null;
        }

        function onTouchStart(event) {
            if (disposed) return;
            var ownsActiveStream = gesture !== null && gesture.state !== "Selected";
            if (!ownsActiveStream && !isScreenTouch(event)) return;
            if (!isTrustedTouch(event)) {
                containIgnoredTouch(event);
                return;
            }
            if (!consumeTouch(event, !ownsActiveStream)) return;
            var retainedSelection = gesture !== null && gesture.state === "Selected";
            if (gesture !== null && !retainedSelection) {
                blockGesture(event);
                return;
            }
            if (event.touches.length !== 1 || event.changedTouches.length !== 1) {
                failPage();
                return;
            }
            var touch = event.changedTouches[0];
            touch = findTouch(event.touches, touch.identifier);
            if (touch === null || !touchIsInsideScreen(touch)) {
                failPage();
                return;
            }
            if (!retainedSelection && compositionActive) {
                gesture = {
                    state: "Composing",
                    identifier: touch.identifier,
                    frame: null,
                    timer: null
                };
                return;
            }
            var bounds = ownerScreen.getBoundingClientRect();
            var rows = ownerTerminal.rows;
            var rowHeight = bounds.height / rows;
            if (!Number.isFinite(rowHeight) || rowHeight <= 0) {
                failPage();
                return;
            }
            var horizontalInset = Math.min(0.5, bounds.width / 2);
            var verticalInset = Math.min(0.5, bounds.height / 2);
            var pending = {
                state: retainedSelection ? "PendingSelected" : "Pending",
                identifier: touch.identifier,
                startX: touch.clientX,
                startY: touch.clientY,
                previousY: touch.clientY,
                clientX: Math.max(bounds.left + horizontalInset,
                    Math.min(bounds.right - horizontalInset, touch.clientX)),
                clientY: Math.max(bounds.top + verticalInset,
                    Math.min(bounds.bottom - verticalInset, touch.clientY)),
                rowHeight: rowHeight,
                accumulator: 0,
                emittedRows: 0,
                frame: null,
                timer: null
            };
            gesture = pending;
            pending.timer = window.setTimeout(function () {
                if (gesture === pending) startSelection(pending);
            }, longPressMilliseconds);
        }

        function onTouchMove(event) {
            if (disposed) return;
            var active = gesture;
            if (active === null && !isScreenTouch(event)) return;
            if (!isTrustedTouch(event)) {
                containIgnoredTouch(event);
                return;
            }
            if (active === null) {
                containIgnoredTouch(event);
                return;
            }
            if (!consumeTouch(event)) return;
            if (active.state === "Blocked") {
                if (event.touches.length === 0) gesture = null;
                return;
            }
            if (event.touches.length !== 1) {
                blockGesture(event);
                return;
            }
            var touch = findTouch(event.touches, active.identifier);
            if (touch === null) {
                blockGesture(event);
                return;
            }
            if (active.state === "Composing") return;
            if (active.state === "Selecting") {
                extendSelection(active, "Extend", touch.clientX, touch.clientY);
                return;
            }
            if (active.state === "Scrolling" || active.state === "BlockingSelected") {
                if (active.state === "Scrolling" && addMovement(active, touch.clientY)) {
                    scheduleDispatch(active);
                }
                return;
            }
            if (active.state === "Pending" || active.state === "PendingSelected") {
                var deltaX = touch.clientX - active.startX;
                var deltaY = touch.clientY - active.startY;
                var absoluteX = Math.abs(deltaX);
                var absoluteY = Math.abs(deltaY);
                if (absoluteX <= touchSlop && absoluteY <= touchSlop) return;
                clearTimer(active);
                if (active.state === "PendingSelected") {
                    active.state = "BlockingSelected";
                    return;
                }
                if (absoluteY <= absoluteX) {
                    blockGesture(event);
                    return;
                }
                active.state = "Scrolling";
            }
            if (active.state !== "Scrolling") {
                failPage();
                return;
            }
            if (addMovement(active, touch.clientY)) scheduleDispatch(active);
        }

        function onTouchEnd(event) {
            if (disposed) return;
            var active = gesture;
            if (active === null && !isScreenTouch(event)) return;
            if (!isTrustedTouch(event)) {
                containIgnoredTouch(event);
                return;
            }
            if (active === null) {
                containIgnoredTouch(event);
                return;
            }
            if (!consumeTouch(event)) return;
            if (active.state === "Blocked") {
                if (event.touches.length === 0) gesture = null;
                return;
            }
            if (event.touches.length !== 0 || event.changedTouches.length !== 1) {
                blockGesture(event);
                return;
            }
            var touch = findTouch(event.changedTouches, active.identifier);
            if (touch === null || !Number.isFinite(touch.clientX) || !Number.isFinite(touch.clientY)) {
                blockGesture(event);
                return;
            }
            if (active.state === "Composing") {
                gesture = null;
                return;
            }
            if (active.state === "Pending") {
                clearTimer(active);
                gesture = null;
                focusTerminal();
                if (pageFailed) return;
                try {
                    ownerTerminal.handleTapInput({ clientX: touch.clientX, clientY: touch.clientY });
                } catch (error) {
                    failPage();
                    return;
                }
                send({ kind: "ImeRequested" });
                return;
            }
            if (active.state === "PendingSelected") {
                clearTimer(active);
                gesture = null;
                ownerTerminal.clearSelection();
                acknowledgeSelectionCleared();
                return;
            }
            if (active.state === "Selecting") {
                releaseSelection(active, touch);
                return;
            }
            if (active.state === "BlockingSelected") {
                gesture = { state: "Selected", frame: null, timer: null };
                return;
            }
            if (active.state !== "Scrolling") {
                failPage();
                return;
            }
            clearFrame(active);
            var rowLimit = ownerTerminal.rows;
            if (!Number.isFinite(rowLimit) || rowLimit <= 0) {
                cancelGesture(true);
                return;
            }
            var targetRows = wholeRows(active.startY - touch.clientY, active.rowHeight);
            targetRows = Math.max(-rowLimit, Math.min(rowLimit, targetRows));
            var correctionRows = Math.max(
                -rowLimit,
                Math.min(rowLimit, targetRows - active.emittedRows)
            );
            if (correctionRows !== 0) {
                sendWheelLines(correctionRows, active.clientX, active.clientY);
            }
            gesture = null;
            active.accumulator = 0;
            active.emittedRows = 0;
        }

        function onTouchCancel(event) {
            if (disposed) return;
            if (gesture === null && !isScreenTouch(event)) return;
            if (!isTrustedTouch(event)) {
                containIgnoredTouch(event);
                return;
            }
            if (gesture === null) {
                containIgnoredTouch(event);
                return;
            }
            if (!consumeTouch(event)) return;
            blockGesture(event);
        }

        function onInputBlur() {
            if (gesture !== null && gesture.state !== "Selected") cancelGesture(true);
        }

        function cancelActiveGesture() {
            if (gesture !== null && gesture.state !== "Selected") cancelGesture(true);
        }

        function onVisibilityChange() {
            if (document.visibilityState === "hidden") cancelGesture(true);
        }

        function scroll(direction) {
            if (disposed) return;
            if (selectionGeneration !== null && !selectionGeneration.clearAcknowledged) {
                cancelGesture(true);
                return;
            }
            if (ownerTerminal.hasSelection()) {
                failPage();
                return;
            }
            cancelGesture(true);
            var bounds = ownerScreen.getBoundingClientRect();
            var rows = ownerTerminal.rows;
            if (!Number.isFinite(bounds.width) || !Number.isFinite(bounds.height) ||
                bounds.width <= 0 || bounds.height <= 0 ||
                !Number.isFinite(rows) || rows <= 0) return;
            var magnitude = Math.max(1, rows - 1);
            sendWheelLines(
                direction === "Backward" ? -magnitude : magnitude,
                bounds.left + bounds.width / 2,
                bounds.top + bounds.height / 2
            );
        }

        function dispose() {
            if (disposed) return;
            cancelGesture(false);
            disposed = true;
            window.removeEventListener("touchstart", onTouchStart, true);
            window.removeEventListener("touchmove", onTouchMove, true);
            window.removeEventListener("touchend", onTouchEnd, true);
            window.removeEventListener("touchcancel", onTouchCancel, true);
            if (ownerInput) ownerInput.removeEventListener("blur", onInputBlur, true);
            window.removeEventListener("blur", cancelGesture, true);
            window.removeEventListener("resize", cancelActiveGesture, true);
            window.removeEventListener("orientationchange", cancelActiveGesture, true);
            window.removeEventListener("pagehide", dispose, true);
            document.removeEventListener("visibilitychange", onVisibilityChange, true);
        }

        window.addEventListener("touchstart", onTouchStart, { capture: true, passive: false });
        window.addEventListener("touchmove", onTouchMove, { capture: true, passive: false });
        window.addEventListener("touchend", onTouchEnd, { capture: true, passive: false });
        window.addEventListener("touchcancel", onTouchCancel, { capture: true, passive: false });
        if (ownerInput) ownerInput.addEventListener("blur", onInputBlur, true);
        window.addEventListener("blur", cancelGesture, true);
        window.addEventListener("resize", cancelActiveGesture, true);
        window.addEventListener("orientationchange", cancelActiveGesture, true);
        window.addEventListener("pagehide", dispose, true);
        document.addEventListener("visibilitychange", onVisibilityChange, true);

        return {
            cancel: function () { cancelGesture(true); },
            clearSelection: clearSelectionFromNative,
            scroll: scroll,
            dispose: dispose
        };
    }

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

    // Geometry stays local until the vendored faces settle, so the grid is
    // measured in the real font; a rejected load degrades to monospace rather
    // than withholding geometry.
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
        if (pageFailed) return;
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
        if (payload.kind === "ResetInputState" && exactObject(payload, ["kind"])) {
            resetInputState();
            return;
        }
        if (payload.kind === "FontSize" && exactObject(payload, ["kind", "fontSizeCssPx"]) &&
            isFontSizeCssPx(payload.fontSizeCssPx)) {
            terminal.options.fontSize = payload.fontSizeCssPx;
            scheduleFit();
            return;
        }
        if (payload.kind === "Scroll" && exactObject(payload, ["kind", "direction"]) &&
            (payload.direction === "Backward" || payload.direction === "Forward")) {
            clearProvenKey();
            terminalTouchInteraction.scroll(payload.direction);
            return;
        }
        if (payload.kind === "ClearSelection" && exactObject(payload, ["kind", "generation"]) &&
            typeof payload.generation === "string") {
            clearProvenKey();
            terminalTouchInteraction.clearSelection(payload.generation);
            return;
        }
        failPage();
    }

    function acceptPagePort(event) {
        var handshake = parseObject(event.data);
        var validHandshake = event.ports && event.ports.length === 1 &&
            handshake &&
            exactObject(handshake, ["kind", "version", "longPressMilliseconds", "fontSizeCssPx"]) &&
            handshake.kind === "PagePort" && handshake.version === 4 &&
            typeof handshake.longPressMilliseconds === "number" &&
            Number.isFinite(handshake.longPressMilliseconds) &&
            Number.isInteger(handshake.longPressMilliseconds) &&
            handshake.longPressMilliseconds > 0 &&
            isFontSizeCssPx(handshake.fontSizeCssPx);
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
        terminal.options.fontSize = handshake.fontSizeCssPx;
        terminalTouchInteraction = createTerminalTouchInteraction({
            terminal: terminal,
            screen: screen,
            longPressMilliseconds: handshake.longPressMilliseconds
        });
        pagePort.onmessage = acceptNativeMessage;
        pagePort.onmessageerror = failPage;
        pagePort.start();
        publishModifiers();
        send({ kind: "Ready" });
        scheduleFit();
    }
}());
