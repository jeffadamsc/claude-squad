import { useEffect, useRef, useState, useCallback } from "react";
import { Terminal } from "@xterm/xterm";
import { AttachAddon } from "@xterm/addon-attach";
import { FitAddon } from "@xterm/addon-fit";
import { theme } from "../lib/theme";
import { useSessionStore } from "../store/sessionStore";

interface UseTerminalOptions {
  sessionId: string;
  wsPort: number;
}

const INITIAL_RECONNECT_DELAY = 1000;
const MAX_RECONNECT_DELAY = 10000;

export function useTerminal(
  containerRef: React.RefObject<HTMLDivElement | null>,
  options: UseTerminalOptions
) {
  const termRef = useRef<Terminal | null>(null);
  const wsRef = useRef<WebSocket | null>(null);
  const fitRef = useRef<FitAddon | null>(null);
  const attachRef = useRef<AttachAddon | null>(null);
  const reconnectTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const reconnectDelay = useRef(INITIAL_RECONNECT_DELAY);
  const intentionalClose = useRef(false);
  const userScrolledUp = useRef(false);
  const programmaticScroll = useRef(false);
  const [disconnected, setDisconnected] = useState(false);
  const terminalFontSize = useSessionStore((s) => s.terminalFontSize);

  // Scroll terminal to bottom robustly. xterm sometimes needs more than one
  // animation frame to finish rendering (e.g. after fit or large writes), so
  // we issue scrollToBottom across two consecutive frames. We flag these as
  // programmatic so the onScroll handler doesn't confuse them with user input.
  const scrollToBottom = useCallback((term: Terminal) => {
    requestAnimationFrame(() => {
      programmaticScroll.current = true;
      term.scrollToBottom();
      requestAnimationFrame(() => {
        programmaticScroll.current = true;
        term.scrollToBottom();
      });
    });
  }, []);

  // Check if the terminal viewport is currently scrolled to the bottom
  const isAtBottom = useCallback((term: Terminal) => {
    const buf = term.buffer.active;
    return buf.viewportY >= buf.baseY;
  }, []);

  const connect = useCallback(() => {
    const container = containerRef.current;
    if (!container || !options.sessionId || !options.wsPort) return;

    let term = termRef.current;
    const fit = fitRef.current;

    // Create terminal if it doesn't exist yet
    if (!term) return;

    const ws = new WebSocket(
      `ws://127.0.0.1:${options.wsPort}/ws/${options.sessionId}`
    );
    ws.binaryType = "arraybuffer";

    ws.onopen = () => {
      // Detach old addon if any
      if (attachRef.current) {
        attachRef.current.dispose();
        attachRef.current = null;
      }

      const attach = new AttachAddon(ws);
      term!.loadAddon(attach);
      attachRef.current = attach;

      reconnectDelay.current = INITIAL_RECONNECT_DELAY;
      setDisconnected(false);

      if (fit) {
        const dims = fit.proposeDimensions();
        if (dims) {
          ws.send(
            JSON.stringify({ type: "resize", rows: dims.rows, cols: dims.cols })
          );
        }
      }

      // After reconnection, the server replays the snapshot which can leave
      // the viewport scrolled to an arbitrary position. Reset scrolled-up
      // state and force scroll during replay.
      if (term) {
        userScrolledUp.current = false;
        const renderDispose = term.onRender(() => {
          term!.scrollToBottom();
        });
        setTimeout(() => renderDispose.dispose(), 500);
      }
    };

    ws.onclose = () => {
      if (intentionalClose.current) return;
      setDisconnected(true);
      scheduleReconnect();
    };

    ws.onerror = () => {
      // onclose will fire after this, which handles reconnect
    };

    wsRef.current = ws;
  }, [options.sessionId, options.wsPort, containerRef, scrollToBottom]);

  const scheduleReconnect = useCallback(() => {
    if (reconnectTimer.current) return;
    const delay = reconnectDelay.current;
    reconnectDelay.current = Math.min(delay * 2, MAX_RECONNECT_DELAY);
    reconnectTimer.current = setTimeout(() => {
      reconnectTimer.current = null;
      connect();
    }, delay);
  }, [connect]);

  useEffect(() => {
    const container = containerRef.current;
    if (!container || !options.sessionId || !options.wsPort) return;

    intentionalClose.current = false;

    const term = new Terminal({
      cursorBlink: true,
      fontFamily: "'JetBrains Mono', 'Fira Code', 'Cascadia Code', monospace",
      fontSize: useSessionStore.getState().terminalFontSize,
      theme: {
        background: theme.crust,
        foreground: theme.text,
        cursor: theme.yellow,
        selectionBackground: theme.surface2,
      },
    });

    const fit = new FitAddon();
    term.loadAddon(fit);
    term.open(container);
    fit.fit();

    termRef.current = term;
    fitRef.current = fit;

    // Initial connection
    const ws = new WebSocket(
      `ws://127.0.0.1:${options.wsPort}/ws/${options.sessionId}`
    );
    ws.binaryType = "arraybuffer";

    ws.onopen = () => {
      const attach = new AttachAddon(ws);
      term.loadAddon(attach);
      attachRef.current = attach;

      reconnectDelay.current = INITIAL_RECONNECT_DELAY;
      setDisconnected(false);

      const dims = fit.proposeDimensions();
      if (dims) {
        ws.send(
          JSON.stringify({ type: "resize", rows: dims.rows, cols: dims.cols })
        );
      }

      // After initial connection, the server replays the snapshot which can
      // leave the viewport scrolled to an arbitrary position. Reset
      // scrolled-up state and keep scrolling to bottom until data settles.
      userScrolledUp.current = false;
      const renderDispose = term.onRender(() => {
        term.scrollToBottom();
      });
      setTimeout(() => renderDispose.dispose(), 500);
    };

    ws.onclose = () => {
      if (intentionalClose.current) return;
      setDisconnected(true);
      scheduleReconnect();
    };

    ws.onerror = () => {};

    wsRef.current = ws;

    // Track whether the user has intentionally scrolled up. When they're
    // at the bottom we auto-scroll on new output; when they scroll up we
    // leave them alone until they return to the bottom. We skip
    // programmatic scrolls to avoid resetting the flag incorrectly.
    const scrollDispose = term.onScroll(() => {
      if (programmaticScroll.current) {
        programmaticScroll.current = false;
        return;
      }
      userScrolledUp.current = !isAtBottom(term);
    });

    // Whenever xterm finishes parsing a write, scroll to bottom if the user
    // hasn't scrolled up. This keeps the viewport pinned during normal
    // output without fighting a user who is reading history.
    const writeDispose = term.onWriteParsed(() => {
      if (!userScrolledUp.current) {
        programmaticScroll.current = true;
        term.scrollToBottom();
      }
    });

    const resizeObserver = new ResizeObserver(() => {
      // Skip resize when container is hidden (display:none gives 0 dimensions)
      if (!container.offsetWidth || !container.offsetHeight) return;
      const wasAtBottom = isAtBottom(term) || !userScrolledUp.current;
      fit.fit();
      const dims = fit.proposeDimensions();
      const currentWs = wsRef.current;
      if (dims && dims.rows > 0 && dims.cols > 0 && currentWs && currentWs.readyState === WebSocket.OPEN) {
        currentWs.send(
          JSON.stringify({ type: "resize", rows: dims.rows, cols: dims.cols })
        );
      }
      // Refit after resize can leave viewport at a stale scroll position.
      // Restore scroll-to-bottom if we were already there.
      if (wasAtBottom) {
        userScrolledUp.current = false;
        scrollToBottom(term);
      }
    });
    resizeObserver.observe(container);

    return () => {
      intentionalClose.current = true;
      scrollDispose.dispose();
      writeDispose.dispose();
      if (reconnectTimer.current) {
        clearTimeout(reconnectTimer.current);
        reconnectTimer.current = null;
      }
      resizeObserver.disconnect();
      if (wsRef.current) {
        wsRef.current.close();
        wsRef.current = null;
      }
      if (attachRef.current) {
        attachRef.current.dispose();
        attachRef.current = null;
      }
      term.dispose();
      termRef.current = null;
      fitRef.current = null;
    };
  }, [options.sessionId, options.wsPort, containerRef, connect, scheduleReconnect]);

  // Sync font size changes to the live terminal instance
  useEffect(() => {
    const term = termRef.current;
    const fit = fitRef.current;
    if (!term) return;
    const wasAtBottom = isAtBottom(term) || !userScrolledUp.current;
    term.options.fontSize = terminalFontSize;
    if (fit) {
      fit.fit();
      const dims = fit.proposeDimensions();
      const ws = wsRef.current;
      if (dims && dims.rows > 0 && dims.cols > 0 && ws && ws.readyState === WebSocket.OPEN) {
        ws.send(JSON.stringify({ type: "resize", rows: dims.rows, cols: dims.cols }));
      }
    }
    if (wasAtBottom) {
      userScrolledUp.current = false;
      scrollToBottom(term);
    }
  }, [terminalFontSize, isAtBottom, scrollToBottom]);

  return { termRef, wsRef, fitRef, disconnected };
}
