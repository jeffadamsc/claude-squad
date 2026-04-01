import { useState, useCallback, useEffect } from "react";
import { TerminalPane } from "../Terminal/TerminalPane";
import { api } from "../../lib/wails";

interface ClaudeTerminalProps {
  sessionId: string;
  ptyId: string;
  wsPort: number;
}

type TerminalTab = "claude" | "shell";

export function ClaudeTerminal({
  sessionId,
  ptyId,
  wsPort,
}: ClaudeTerminalProps) {
  const [activeTab, setActiveTab] = useState<TerminalTab>("claude");
  const [shellPtyId, setShellPtyId] = useState<string | null>(null);
  const [spawning, setSpawning] = useState(false);

  const spawnShell = useCallback(async () => {
    if (shellPtyId || spawning) return;
    setSpawning(true);
    try {
      const id = await api().SpawnShell(sessionId);
      setShellPtyId(id);
    } catch (err) {
      console.error("Failed to spawn shell:", err);
    } finally {
      setSpawning(false);
    }
  }, [sessionId, shellPtyId, spawning]);

  // Clean up shell PTY on unmount
  useEffect(() => {
    return () => {
      if (shellPtyId) {
        api().KillShell(shellPtyId).catch(console.error);
      }
    };
  }, [shellPtyId]);

  const handleShellTab = useCallback(() => {
    if (!shellPtyId && !spawning) {
      spawnShell().then(() => setActiveTab("shell"));
    } else {
      setActiveTab("shell");
    }
  }, [shellPtyId, spawning, spawnShell]);

  // Ctrl+` to toggle between Claude and Shell terminal
  useEffect(() => {
    const handler = (e: KeyboardEvent) => {
      if ((e.ctrlKey || e.metaKey) && e.key === "`") {
        e.preventDefault();
        if (activeTab === "claude") {
          handleShellTab();
        } else {
          setActiveTab("claude");
        }
      }
    };
    document.addEventListener("keydown", handler);
    return () => document.removeEventListener("keydown", handler);
  }, [activeTab, handleShellTab]);

  return (
    <div
      style={{
        display: "flex",
        flexDirection: "column",
        height: "100%",
        background: "var(--crust)",
      }}
    >
      <div
        style={{
          display: "flex",
          alignItems: "center",
          borderBottom: "1px solid var(--surface0)",
          background: "var(--base)",
          fontSize: 11,
          color: "var(--subtext0)",
          textTransform: "uppercase",
        }}
      >
        <div
          onClick={() => setActiveTab("claude")}
          style={{
            padding: "8px 10px",
            display: "flex",
            alignItems: "center",
            gap: 6,
            cursor: "pointer",
            borderBottom:
              activeTab === "claude"
                ? "2px solid var(--blue)"
                : "2px solid transparent",
            color:
              activeTab === "claude" ? "var(--text)" : "var(--overlay0)",
          }}
        >
          <span style={{ color: "#cba6f7" }}>{"\u2B24"}</span>
          Claude
        </div>
        <div
          onClick={handleShellTab}
          style={{
            padding: "8px 10px",
            display: "flex",
            alignItems: "center",
            gap: 6,
            cursor: "pointer",
            borderBottom:
              activeTab === "shell"
                ? "2px solid var(--blue)"
                : "2px solid transparent",
            color:
              activeTab === "shell" ? "var(--text)" : "var(--overlay0)",
          }}
        >
          <span style={{ fontSize: 10 }}>{"\u25B6"}</span>
          Terminal
        </div>
      </div>
      <div style={{ flex: 1, position: "relative" }}>
        <div
          style={{
            position: "absolute",
            inset: 0,
            display: activeTab === "claude" ? "block" : "none",
          }}
        >
          <TerminalPane
            sessionId={ptyId}
            wsPort={wsPort}
            focused={activeTab === "claude"}
            instanceId={sessionId}
          />
        </div>
        {shellPtyId && (
          <div
            style={{
              position: "absolute",
              inset: 0,
              display: activeTab === "shell" ? "block" : "none",
            }}
          >
            <TerminalPane
              sessionId={shellPtyId}
              wsPort={wsPort}
              focused={activeTab === "shell"}
              instanceId={`${sessionId}-shell`}
            />
          </div>
        )}
        {activeTab === "shell" && !shellPtyId && spawning && (
          <div
            style={{
              position: "absolute",
              inset: 0,
              display: "flex",
              alignItems: "center",
              justifyContent: "center",
              color: "var(--overlay0)",
              fontSize: 13,
            }}
          >
            Starting shell...
          </div>
        )}
      </div>
    </div>
  );
}
