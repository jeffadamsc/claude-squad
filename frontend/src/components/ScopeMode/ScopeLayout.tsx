import { useEffect } from "react";
import { Allotment } from "allotment";
import "allotment/dist/style.css";
import { useSessionStore } from "../../store/sessionStore";
import { ScopeSidebar } from "./ScopeSidebar";
import { FileExplorer } from "./FileExplorer";
import { EditorPane } from "./EditorPane";
import { ClaudeTerminal } from "./ClaudeTerminal";
import { QuickOpen } from "./QuickOpen";
import { api } from "../../lib/wails";

function SearchBar() {
  const toggleQuickOpen = useSessionStore((s) => s.toggleQuickOpen);

  return (
    <div
      style={{
        height: 32,
        minHeight: 32,
        display: "flex",
        alignItems: "center",
        justifyContent: "center",
        background: "var(--base)",
        borderBottom: "1px solid var(--surface0)",
        padding: "0 16px",
      }}
    >
      <div
        onClick={toggleQuickOpen}
        style={{
          width: 360,
          height: 22,
          background: "var(--surface0)",
          border: "1px solid var(--surface1)",
          borderRadius: 6,
          display: "flex",
          alignItems: "center",
          justifyContent: "center",
          gap: 6,
          cursor: "pointer",
          fontSize: 12,
          color: "var(--overlay0)",
          userSelect: "none",
        }}
      >
        <span style={{ fontSize: 11 }}>Search files by name...</span>
        <span
          style={{
            fontSize: 10,
            color: "var(--overlay0)",
            background: "var(--surface1)",
            borderRadius: 3,
            padding: "1px 5px",
          }}
        >
          {navigator.platform.includes("Mac") ? "\u2318P" : "Ctrl+P"}
        </span>
      </div>
    </div>
  );
}

interface ScopeLayoutProps {
  wsPort: number;
}

export function ScopeLayout({ wsPort }: ScopeLayoutProps) {
  const scopeMode = useSessionStore((s) => s.scopeMode);
  const tabs = useSessionStore((s) => s.tabs);
  const sessions = useSessionStore((s) => s.sessions);
  const exitScopeMode = useSessionStore((s) => s.exitScopeMode);
  const setFileList = useSessionStore((s) => s.setFileList);

  const sessionId = scopeMode.sessionId;
  const tab = tabs.find((t) => t.sessionId === sessionId);

  // Exit scope mode if scoped session is removed
  useEffect(() => {
    if (sessionId && !sessions.find((s) => s.id === sessionId)) {
      exitScopeMode();
    }
  }, [sessions, sessionId, exitScopeMode]);

  // Load file list and start indexer on scope entry
  useEffect(() => {
    if (!sessionId) return;

    const loadFiles = () =>
      api()
        .ListFiles(sessionId)
        .then((files) => {
          if (files && files.length > 0) setFileList(files);
        })
        .catch(console.error);

    // Load files immediately (before indexer, so the user sees results fast)
    loadFiles();

    // Start indexer in the background, then reload files once it's ready
    api()
      .IndexSession(sessionId)
      .then(() => loadFiles())
      .catch(console.error);

    const interval = setInterval(loadFiles, 10000);

    return () => {
      clearInterval(interval);
      api().StopIndexer(sessionId).catch(console.error);
    };
  }, [sessionId, setFileList]);

  if (!sessionId || !tab) {
    return (
      <div
        style={{
          flex: 1,
          display: "flex",
          alignItems: "center",
          justifyContent: "center",
          color: "var(--overlay0)",
        }}
      >
        Session not found. Press Ctrl+Shift+S to exit scope mode.
      </div>
    );
  }

  return (
    <div style={{ display: "flex", flexDirection: "column", height: "100%", flex: 1 }}>
      <SearchBar />
      <div style={{ display: "flex", flex: 1, minHeight: 0 }}>
        <ScopeSidebar />
        {sessionId && <QuickOpen sessionId={sessionId} />}
        <Allotment>
          <Allotment.Pane preferredSize={200} minSize={150} maxSize={350}>
            <FileExplorer sessionId={sessionId} />
          </Allotment.Pane>
          <Allotment.Pane>
            <EditorPane sessionId={sessionId} />
          </Allotment.Pane>
          <Allotment.Pane preferredSize={300} minSize={200} maxSize={500}>
            <ClaudeTerminal
              sessionId={sessionId}
              ptyId={tab.ptyId}
              wsPort={wsPort}
            />
          </Allotment.Pane>
        </Allotment>
      </div>
    </div>
  );
}
