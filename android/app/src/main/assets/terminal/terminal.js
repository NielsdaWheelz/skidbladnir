(function () {
    "use strict";

    var terminalHost = document.getElementById("terminal");
    var pagePort = null;
    var pageFailed = false;
    var fitScheduled = false;
    var fontsSettled = false;
    var modifiers = { control: "Off", alt: "Off", shift: "Off" };
    var pendingProvenKey = null;
    var compositionActive = false;
    var maximumInputBytes = 1024 * 1024;
    var maximumSelectionBytes = 256 * 1024;
    // The gateway's Resize bounds; the page fits down to them and never up.
    var minimumColumns = 20;
    var maximumColumns = 1024;
    var minimumRows = 5;
    var maximumRows = 512;
    var lastPublishedColumns = 0;
    var lastPublishedRows = 0;
    var viewportTooSmallPublished = false;
    var terminal = null;
    // Touch gesture state (section "touch gestures"); live from the handshake.
    var gesture = null;
    var longPressMilliseconds = 0;
    var selectionGeneration = null;
    var lastAcknowledgedSelectionGeneration = null;
    var nextSelectionGeneration = 1;

    window.addEventListener("message", acceptPagePort);
    if (!terminalHost) {
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
        // Not for screen readers: this selects xterm's keydown and insertText input
        // path. With it off, Gboard lost the first letter in 4 of 72 emulator runs.
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
        cancelGesture(false);
        clearProvenKey();
        setModifiers("Off", "Off", "Off");
        send({ kind: "PageFailure" });
        pagePort = null;
    }

    function publishModifiers() {
        send({
            kind: "ModifierState",
            control: modifiers.control,
            alt: modifiers.alt,
            shift: modifiers.shift
        });
    }

    function setModifiers(control, alt, shift) {
        if (modifiers.control === control && modifiers.alt === alt && modifiers.shift === shift) return;
        modifiers = { control: control, alt: alt, shift: shift };
        publishModifiers();
    }

    function resetInputState() {
        clearProvenKey();
        cancelGesture(true);
        setModifiers("Off", "Off", "Off");
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
        setModifiers("Off", "Off", "Off");
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

    // Touch gestures. One prevented, trusted TouchEvent stream owns tap, scroll
    // and selection: Idle (gesture null) | Composing | Pending | Scrolling |
    // Selecting | Selected | PendingSelected | BlockingSelected | Blocked.
    var touchSlop = 8;

    function cellAt(clientX, clientY) {
        var bounds = screen.getBoundingClientRect();
        return {
            col: Math.max(0, Math.min(terminal.cols - 1,
                Math.floor((clientX - bounds.left) * terminal.cols / bounds.width))),
            row: Math.max(0, Math.min(terminal.rows - 1,
                Math.floor((clientY - bounds.top) * terminal.rows / bounds.height)))
        };
    }

    // The phone renders a tmux client, and tmux negotiates SGR (1006) with every
    // mouse mode it enables, so reports are SGR. Reports and keys enter through
    // terminal.input like typed input: the input reducer consumes armed
    // modifiers without applying them.
    //
    // One wheel unit of signed whole rows (negative = backward) at a cell. A
    // remote unit is one report or one key whatever |rows|, so finger speed
    // never multiplies terminal input.
    function sendWheel(rows, cell) {
        var modes = terminal.modes;
        var tracking = modes.mouseTrackingMode;
        if (tracking === "vt200" || tracking === "drag" || tracking === "any") {
            terminal.input("\u001b[<" + (rows < 0 ? 64 : 65) + ";" + (cell.col + 1) + ";" +
                (cell.row + 1) + "M", true);
        } else if (terminal.buffer.active.type === "alternate") {
            terminal.input("\u001b" + (modes.applicationCursorKeysMode ? "O" : "[") +
                (rows < 0 ? "A" : "B"), true);
        } else {
            terminal.scrollLines(rows);
        }
    }

    // A primary click: nothing without mouse tracking, press only under X10.
    function sendTap(cell) {
        var tracking = terminal.modes.mouseTrackingMode;
        if (tracking === "none") return;
        var position = ";" + (cell.col + 1) + ";" + (cell.row + 1);
        terminal.input("\u001b[<0" + position + "M", true);
        if (tracking !== "x10") terminal.input("\u001b[<0" + position + "m", true);
    }

    // xterm's default word separators; an empty cell's "" matches too.
    var wordSeparators = " ()[]{}',\"`";

    // Buffer cells are indexed row * cols + col; ranges are [start, end).
    function isWordCell(buffer, index) {
        var line = buffer.getLine(Math.floor(index / terminal.cols));
        var cell = line && line.getCell(index % terminal.cols);
        return cell !== undefined &&
            (cell.getWidth() === 0 || wordSeparators.indexOf(cell.getChars()) < 0);
    }

    function isSoftWrapped(buffer, row) {
        var line = buffer.getLine(row);
        return line !== undefined && line.isWrapped;
    }

    // xterm's word at a cell: the cell alone when blank, else the run of
    // non-separator cells through it, continued across soft wraps. Wide cells
    // stay whole.
    function wordAt(cell) {
        var buffer = terminal.buffer.active;
        var cols = terminal.cols;
        var row = buffer.viewportY + cell.row;
        var line = buffer.getLine(row);
        var col = cell.col;
        if (col > 0 && line.getCell(col).getWidth() === 0) col -= 1;
        var lead = line.getCell(col);
        var start = row * cols + col;
        var end = start + Math.max(1, lead.getWidth());
        if (lead.getChars().trim() === "") return { start: start, end: end };
        while (start > 0 && (start % cols !== 0 || isSoftWrapped(buffer, start / cols)) &&
            isWordCell(buffer, start - 1)) start -= 1;
        while ((end % cols !== 0 || isSoftWrapped(buffer, end / cols)) &&
            isWordCell(buffer, end)) end += 1;
        return { start: start, end: end };
    }

    // The word stays anchored and the finger's cell extends it either way; a
    // finger above or below the screen reaches that edge row. The anchor is a
    // marker on the word's first row, so it follows the text as output scrolls
    // the buffer, through the row a full buffer trims per scrolled line.
    // Re-applied on every move, tick and release: xterm clears its selection
    // whenever tmux re-sends its mouse modes. False once the word's row has
    // left the buffer or its buffer is no longer active.
    function applySelection(active) {
        var buffer = terminal.buffer.active;
        if (active.anchor.isDisposed || buffer.type !== active.anchorBuffer) return false;
        var cols = terminal.cols;
        var wordStart = active.anchor.line * cols + active.word.start;
        var wordEnd = active.anchor.line * cols + active.word.end;
        var bounds = screen.getBoundingClientRect();
        var end;
        if (active.y > bounds.bottom) {
            end = (buffer.viewportY + terminal.rows) * cols;
        } else if (active.y < bounds.top) {
            end = buffer.viewportY * cols;
        } else {
            var cell = cellAt(active.x, active.y);
            var row = buffer.viewportY + cell.row;
            var line = buffer.getLine(row);
            var col = cell.col;
            while (col > 0 && line.getCell(col).getWidth() === 0) col -= 1;
            end = row * cols + col;
            if (end >= wordStart) end += Math.max(1, line.getCell(col).getWidth());
        }
        var start = Math.min(end, wordStart);
        var stop = end < wordStart ? wordEnd : Math.max(end, wordEnd);
        terminal.select(start % cols, Math.floor(start / cols), stop - start);
        return true;
    }

    // Every 50 ms while selecting: a finger beyond the top or bottom edge
    // scrolls history 1..15 rows by its distance, as xterm's mouse selection
    // does, and the range is re-applied, so a resting finger's highlight
    // follows its anchor and survives mode resets.
    function tickSelection(active) {
        var bounds = screen.getBoundingClientRect();
        var beyond = active.y < bounds.top ? active.y - bounds.top :
            active.y > bounds.bottom ? active.y - bounds.bottom : 0;
        var reach = Math.max(-50, Math.min(50, beyond)) / 50;
        if (reach !== 0) terminal.scrollLines(Math.sign(reach) + Math.round(reach * 14));
        if (!applySelection(active)) cancelGesture(true);
    }

    function beginSelectionGeneration() {
        if (selectionGeneration !== null && !selectionGeneration.clearAcknowledged) {
            failPage();
            return null;
        }
        var generation = String(nextSelectionGeneration);
        nextSelectionGeneration += 1;
        selectionGeneration = { value: generation, clearAcknowledged: false };
        return generation;
    }

    function acknowledgeSelectionCleared() {
        if (selectionGeneration === null) {
            failPage();
            return false;
        }
        if (!selectionGeneration.clearAcknowledged) {
            selectionGeneration.clearAcknowledged = true;
            lastAcknowledgedSelectionGeneration = selectionGeneration.value;
            send({ kind: "SelectionCleared", generation: selectionGeneration.value });
        }
        return true;
    }

    function holdsSelection(state) {
        return state === "Selecting" || state === "Selected" ||
            state === "PendingSelected" || state === "BlockingSelected";
    }

    function clearFrame(active) {
        if (active.frame === null) return;
        window.cancelAnimationFrame(active.frame);
        active.frame = null;
    }

    // A gesture's one timer: the long-press timeout while pending, then the
    // selection tick interval while selecting. clearTimeout clears either.
    function clearTimer(active) {
        if (active.timer === null) return;
        window.clearTimeout(active.timer);
        active.timer = null;
    }

    function cancelGesture(blockTail) {
        var active = gesture;
        if (active === null) return;
        clearTimer(active);
        clearFrame(active);
        if (active.state === "Selecting") active.anchor.dispose();
        gesture = blockTail && active.identifier !== undefined ?
            { state: "Blocked", identifier: active.identifier, frame: null, timer: null } : null;
        if (!holdsSelection(active.state)) return;
        terminal.clearSelection();
        acknowledgeSelectionCleared();
    }

    function cancelUnlessSelected() {
        if (gesture !== null && gesture.state !== "Selected") cancelGesture(true);
    }

    function clearSelectionFromNative(generation) {
        if (!/^[1-9][0-9]*$/.test(generation) || !Number.isSafeInteger(Number(generation))) {
            failPage();
            return;
        }
        if (selectionGeneration === null || generation !== selectionGeneration.value) {
            if (lastAcknowledgedSelectionGeneration !== null &&
                Number(generation) <= Number(lastAcknowledgedSelectionGeneration)) return;
            failPage();
            return;
        }
        if (selectionGeneration.clearAcknowledged) return;
        var held = gesture !== null && holdsSelection(gesture.state);
        cancelGesture(true);
        if (pageFailed || held) return;
        terminal.clearSelection();
        acknowledgeSelectionCleared();
    }

    function startSelection(active) {
        clearTimer(active);
        if (active.state === "PendingSelected") {
            terminal.clearSelection();
            if (!acknowledgeSelectionCleared()) return;
        }
        var generation = beginSelectionGeneration();
        if (generation === null) return;
        var buffer = terminal.buffer.active;
        var cols = terminal.cols;
        var word = wordAt(cellAt(active.startX, active.startY));
        var row = Math.floor(word.start / cols);
        active.anchor = terminal.registerMarker(row - buffer.baseY - buffer.cursorY);
        active.anchorBuffer = buffer.type;
        active.word = { start: word.start - row * cols, end: word.end - row * cols };
        active.x = active.startX;
        active.y = active.startY;
        applySelection(active);
        active.state = "Selecting";
        active.timer = window.setInterval(function () {
            tickSelection(active);
        }, 50);
        send({ kind: "SelectionStarted", generation: generation });
    }

    function releaseSelection(active, touch) {
        clearTimer(active);
        active.x = touch.clientX;
        active.y = touch.clientY;
        if (!applySelection(active)) {
            cancelGesture(false);
            return;
        }
        active.anchor.dispose();
        gesture = null;
        var text = terminal.getSelection();
        if (!isUnicodeScalarSequence(text)) {
            terminal.clearSelection();
            failPage();
            return;
        }
        if (text.length === 0) {
            terminal.clearSelection();
            acknowledgeSelectionCleared();
            return;
        }
        if (utf8ByteCount(text, maximumSelectionBytes) === null) {
            terminal.clearSelection();
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
            terminal.clearSelection();
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

    // At most one wheel unit per animation frame: whole rows of accumulated
    // travel bounded to one screen; excess debt is dropped, the fraction kept.
    function scheduleDispatch(active) {
        if (active.frame !== null) return;
        active.frame = window.requestAnimationFrame(function () {
            active.frame = null;
            if (gesture !== active || active.state !== "Scrolling") return;
            if (terminal.hasSelection()) {
                cancelGesture(true);
                return;
            }
            var rows = Math.trunc(active.accumulator / active.rowHeight);
            if (rows === 0) return;
            active.accumulator -= rows * active.rowHeight;
            rows = Math.max(-terminal.rows, Math.min(terminal.rows, rows));
            active.emittedRows += rows;
            sendWheel(rows, active.cell);
        });
    }

    function isScreenTouch(event) {
        var target = event.target;
        return target === screen ||
            (target && target.nodeType === Node.ELEMENT_NODE && screen.contains(target));
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

    function findTouch(list, identifier) {
        var found = null;
        for (var index = 0; index < list.length; index += 1) {
            if (list[index].identifier !== identifier) continue;
            if (found !== null) return null;
            found = list[index];
        }
        return found;
    }

    function blockGesture(event) {
        cancelGesture(true);
        if (gesture === null) gesture = { state: "Blocked", frame: null, timer: null };
        if (event.touches.length === 0) gesture = null;
    }

    function onTouchStart(event) {
        if (pageFailed) return;
        var ownsActiveStream = gesture !== null && gesture.state !== "Selected";
        if (!ownsActiveStream && !isScreenTouch(event)) return;
        if (event.isTrusted !== true) {
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
        var touch = findTouch(event.touches, event.changedTouches[0].identifier);
        var bounds = screen.getBoundingClientRect();
        if (touch === null || !(touch.clientX > bounds.left && touch.clientX < bounds.right &&
            touch.clientY > bounds.top && touch.clientY < bounds.bottom)) {
            failPage();
            return;
        }
        if (!retainedSelection && compositionActive) {
            gesture = { state: "Composing", identifier: touch.identifier, frame: null, timer: null };
            return;
        }
        var rowHeight = bounds.height / terminal.rows;
        if (!Number.isFinite(rowHeight) || rowHeight <= 0) {
            failPage();
            return;
        }
        var pending = {
            state: retainedSelection ? "PendingSelected" : "Pending",
            identifier: touch.identifier,
            startX: touch.clientX,
            startY: touch.clientY,
            previousY: touch.clientY,
            cell: cellAt(touch.clientX, touch.clientY),
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
        if (pageFailed) return;
        var active = gesture;
        if (active === null && !isScreenTouch(event)) return;
        if (event.isTrusted !== true || active === null) {
            containIgnoredTouch(event);
            return;
        }
        if (!consumeTouch(event, false)) return;
        if (active.state === "Blocked") {
            if (event.touches.length === 0) gesture = null;
            return;
        }
        var touch = event.touches.length === 1 ? findTouch(event.touches, active.identifier) : null;
        if (touch === null) {
            blockGesture(event);
            return;
        }
        if (active.state === "Selecting") {
            active.x = touch.clientX;
            active.y = touch.clientY;
            if (!applySelection(active)) cancelGesture(true);
            return;
        }
        if (active.state === "Pending" || active.state === "PendingSelected") {
            var absoluteX = Math.abs(touch.clientX - active.startX);
            var absoluteY = Math.abs(touch.clientY - active.startY);
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
        if (active.state !== "Scrolling") return;
        if (!Number.isFinite(touch.clientY)) {
            cancelGesture(true);
            return;
        }
        active.accumulator += active.previousY - touch.clientY;
        active.previousY = touch.clientY;
        scheduleDispatch(active);
    }

    function onTouchEnd(event) {
        if (pageFailed) return;
        var active = gesture;
        if (active === null && !isScreenTouch(event)) return;
        if (event.isTrusted !== true || active === null) {
            containIgnoredTouch(event);
            return;
        }
        if (!consumeTouch(event, false)) return;
        if (active.state === "Blocked") {
            if (event.touches.length === 0) gesture = null;
            return;
        }
        var touch = event.touches.length === 0 && event.changedTouches.length === 1 ?
            findTouch(event.changedTouches, active.identifier) : null;
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
            sendTap(cellAt(touch.clientX, touch.clientY));
            send({ kind: "ImeRequested" });
            return;
        }
        if (active.state === "PendingSelected") {
            clearTimer(active);
            gesture = null;
            terminal.clearSelection();
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
        // Scrolling (a Selected gesture has no identifier, so it blocked above):
        // one bounded correction to the absolute travel, nothing after release.
        clearFrame(active);
        var target = Math.max(-terminal.rows, Math.min(terminal.rows,
            Math.trunc((active.startY - touch.clientY) / active.rowHeight)));
        var correction = Math.max(-terminal.rows, Math.min(terminal.rows, target - active.emittedRows));
        if (correction !== 0) sendWheel(correction, active.cell);
        gesture = null;
    }

    function onTouchCancel(event) {
        if (pageFailed) return;
        if (gesture === null && !isScreenTouch(event)) return;
        if (event.isTrusted !== true || gesture === null) {
            containIgnoredTouch(event);
            return;
        }
        if (!consumeTouch(event, false)) return;
        blockGesture(event);
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

    // Terminal text leaves the page only in SelectionAvailable; xterm's own
    // element copy listener would write its selection to the clipboard.
    window.addEventListener("copy", function (event) {
        event.stopImmediatePropagation();
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
            Tab: modifiers.shift === "Armed" ? "\u001b[Z" : "\t"
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
        var shiftArmed = modifiers.shift === "Armed";
        var modified = controlArmed || altArmed || shiftArmed;
        var parameter = 1 + (shiftArmed ? 1 : 0) + (altArmed ? 2 : 0) + (controlArmed ? 4 : 0);
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
            setModifiers(modifiers.control === "Off" ? "Armed" : "Off", modifiers.alt, modifiers.shift);
            focusTerminal();
            return;
        }
        if (key === "Alt") {
            setModifiers(modifiers.control, modifiers.alt === "Off" ? "Armed" : "Off", modifiers.shift);
            focusTerminal();
            return;
        }
        if (key === "Shift") {
            setModifiers(modifiers.control, modifiers.alt, modifiers.shift === "Off" ? "Armed" : "Off");
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
        if (payload.kind === "ClearSelection" && exactObject(payload, ["kind", "generation"]) &&
            typeof payload.generation === "string") {
            clearProvenKey();
            clearSelectionFromNative(payload.generation);
            return;
        }
        failPage();
    }

    function acceptPagePort(event) {
        var handshake = parseObject(event.data);
        var validHandshake = event.ports && event.ports.length === 1 &&
            handshake &&
            exactObject(handshake, ["kind", "version", "longPressMilliseconds", "fontSizeCssPx"]) &&
            handshake.kind === "PagePort" && handshake.version === 5 &&
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
        longPressMilliseconds = handshake.longPressMilliseconds;
        window.addEventListener("touchstart", onTouchStart, { capture: true, passive: false });
        window.addEventListener("touchmove", onTouchMove, { capture: true, passive: false });
        window.addEventListener("touchend", onTouchEnd, { capture: true, passive: false });
        window.addEventListener("touchcancel", onTouchCancel, { capture: true, passive: false });
        input.addEventListener("blur", cancelUnlessSelected, true);
        window.addEventListener("blur", function () { cancelGesture(true); }, true);
        window.addEventListener("resize", cancelUnlessSelected, true);
        window.addEventListener("orientationchange", cancelUnlessSelected, true);
        document.addEventListener("visibilitychange", function () {
            if (document.visibilityState === "hidden") cancelGesture(true);
        }, true);
        pagePort.onmessage = acceptNativeMessage;
        pagePort.onmessageerror = failPage;
        pagePort.start();
        publishModifiers();
        send({ kind: "Ready" });
        scheduleFit();
    }
}());
