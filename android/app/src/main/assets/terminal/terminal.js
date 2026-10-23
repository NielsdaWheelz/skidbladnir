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
    var imeContainmentScheduled = false;
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

    window.addEventListener("message", acceptPagePort, { once: true });
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

    // Resets the page once and tells native on whichever port is current.
    function failPage() {
        if (!pageFailed) {
            pageFailed = true;
            cancelGesture(false);
            clearProvenKey();
            setModifiers("Off", "Off", "Off");
        }
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
        if (utf8ByteCount(value, maximumInputBytes) === null) {
            failPage();
            return;
        }
        send({ kind: "Input", value: value });
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

    // Same-APK native messages, read only for what the page uses: a message
    // it cannot parse, an unknown kind or a missing or mistyped field fails
    // the page.
    function parseMessage(data) {
        try {
            return JSON.parse(data);
        } catch (error) {
            return null;
        }
    }

    function keyIsProven(event) {
        var domEvent = event && event.domEvent;
        if (!domEvent || compositionActive || domEvent.isComposing) return false;
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

    // Touch gestures. The page owns every screen touch stream and the selection:
    // Idle (gesture null) | Composing | Pending | Scrolling | Selecting |
    // Selected | PendingSelected | BlockingSelected | Blocked. Every state but
    // Idle and Selected has a finger down, and all but Blocked exactly one.
    // Native hears the selection as SelectionStarted, then one of
    // SelectionAvailable, SelectionTooLarge or SelectionCleared at release, and
    // SelectionCleared whenever a held selection drops for any reason.
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
        gesture = blockTail && active.state !== "Selected" ?
            { state: "Blocked", frame: null, timer: null } : null;
        if (!holdsSelection(active.state)) return;
        terminal.clearSelection();
        send({ kind: "SelectionCleared" });
    }

    function cancelUnlessSelected() {
        if (gesture !== null && gesture.state !== "Selected") cancelGesture(true);
    }

    // Drops the gesture and swallows its stream until every finger lifts.
    function blockGesture(event) {
        cancelGesture(false);
        gesture = event.touches.length === 0 ? null : { state: "Blocked", frame: null, timer: null };
    }

    function startSelection(active) {
        clearTimer(active);
        if (active.state === "PendingSelected") {
            terminal.clearSelection();
            send({ kind: "SelectionCleared" });
        }
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
        send({ kind: "SelectionStarted" });
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
        if (text.length === 0 || utf8ByteCount(text, maximumSelectionBytes) === null) {
            terminal.clearSelection();
            send({ kind: text.length === 0 ? "SelectionCleared" : "SelectionTooLarge" });
            return;
        }
        gesture = { state: "Selected", frame: null, timer: null };
        send({
            kind: "SelectionAvailable",
            anchorX: Math.max(0, Math.min(1, touch.clientX / window.innerWidth)),
            anchorY: Math.max(0, Math.min(1, touch.clientY / window.innerHeight)),
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

    // One handler takes every screen touch stream, and every stream while a
    // gesture or a selection exists, before xterm sees it. A stream the page
    // cannot own whole is blocked until every finger lifts: a second contact,
    // a start during a claimed gesture, a non-cancelable start, or a start not
    // strictly inside the screen (the screen target alone admits the .xterm
    // padding and the screen's border pixels). Touch never fails the page.
    function onTouch(event) {
        var active = gesture;
        var claimed = active !== null && active.state !== "Selected";
        if (active === null && !screen.contains(event.target)) return;
        if (event.cancelable) event.preventDefault();
        event.stopImmediatePropagation();
        if (event.type === "touchstart") {
            var touch = event.touches.length === 1 ? event.touches[0] : null;
            var bounds = screen.getBoundingClientRect();
            if (claimed || touch === null || !event.cancelable ||
                !(touch.clientX > bounds.left && touch.clientX < bounds.right &&
                    touch.clientY > bounds.top && touch.clientY < bounds.bottom)) {
                blockGesture(event);
            } else {
                beginGesture(touch, active !== null, bounds.height / terminal.rows);
            }
            return;
        }
        if (!claimed) return;
        if (active.state === "Blocked") {
            if (event.touches.length === 0) gesture = null;
        } else if (event.type === "touchcancel") {
            blockGesture(event);
        } else if (event.type === "touchmove") {
            moveGesture(active, event.touches[0]);
        } else {
            endGesture(active, event.changedTouches[0]);
        }
    }

    function beginGesture(touch, retainedSelection, rowHeight) {
        if (!retainedSelection && compositionActive) {
            gesture = { state: "Composing", frame: null, timer: null };
            return;
        }
        var pending = {
            state: retainedSelection ? "PendingSelected" : "Pending",
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

    function moveGesture(active, touch) {
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
                cancelGesture(true);
                return;
            }
            active.state = "Scrolling";
        }
        if (active.state !== "Scrolling") return;
        active.accumulator += active.previousY - touch.clientY;
        active.previousY = touch.clientY;
        scheduleDispatch(active);
    }

    function endGesture(active, touch) {
        if (active.state === "Selecting") {
            releaseSelection(active, touch);
        } else if (active.state === "BlockingSelected") {
            gesture = { state: "Selected", frame: null, timer: null };
        } else if (active.state === "Scrolling") {
            // One bounded correction to the absolute travel, nothing after release.
            clearFrame(active);
            var target = Math.max(-terminal.rows, Math.min(terminal.rows,
                Math.trunc((active.startY - touch.clientY) / active.rowHeight)));
            var correction = Math.max(-terminal.rows, Math.min(terminal.rows, target - active.emittedRows));
            if (correction !== 0) sendWheel(correction, active.cell);
            gesture = null;
        } else {
            // Composing ends with no effect and PendingSelected drops the
            // selection; a Pending tap focuses, clicks and asks for the IME.
            cancelGesture(false);
            if (active.state !== "Pending") return;
            focusTerminal();
            sendTap(cellAt(touch.clientX, touch.clientY));
            send({ kind: "ImeRequested" });
        }
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

    // One pass per frame, after the frame's IME events and xterm's own
    // composition layout; the page-set maximum widths persist between passes.
    function scheduleImeContainment() {
        if (imeContainmentScheduled) return;
        imeContainmentScheduled = true;
        window.requestAnimationFrame(function () {
            imeContainmentScheduled = false;
            containImeGeometry();
        });
    }

    function focusTerminal() {
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
        // Output frames arrive as raw bytes; native counts them until xterm has
        // written each one.
        if (message.data instanceof ArrayBuffer) {
            var output = new Uint8Array(message.data);
            terminal.write(output, function () {
                send({ kind: "OutputApplied", bytes: output.length });
            });
            return;
        }
        var payload = parseMessage(message.data);
        var kind = payload && payload.kind;
        if (kind === "Accessory" && typeof payload.key === "string") {
            acceptAccessory(payload.key);
        } else if (kind === "ResetInputState") {
            resetInputState();
        } else if (kind === "FontSize" && typeof payload.fontSizeCssPx === "number") {
            terminal.options.fontSize = payload.fontSizeCssPx;
            scheduleFit();
        } else if (kind === "ClearSelection") {
            // The person closed the Copy toolbar. A selection still being made is
            // newer than any toolbar, so only a released one drops.
            if (gesture !== null && gesture.state !== "Selecting" && holdsSelection(gesture.state)) {
                cancelGesture(true);
            }
        } else {
            failPage();
        }
    }

    // Native posts the one handshake, carrying the port, once the page has
    // loaded. A page that already failed reports it on the new port.
    function acceptPagePort(event) {
        var handshake = parseMessage(event.data);
        pagePort = event.ports[0] || null;
        if (pageFailed || !pagePort || !handshake ||
            typeof handshake.longPressMilliseconds !== "number" || typeof handshake.fontSizeCssPx !== "number") {
            failPage();
            return;
        }
        terminal.options.fontSize = handshake.fontSizeCssPx;
        longPressMilliseconds = handshake.longPressMilliseconds;
        ["touchstart", "touchmove", "touchend", "touchcancel"].forEach(function (name) {
            window.addEventListener(name, onTouch, { capture: true, passive: false });
        });
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
