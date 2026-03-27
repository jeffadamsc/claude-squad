import { useEffect, useRef, useCallback } from "react";
import { useSessionStore, detectLanguage } from "../../store/sessionStore";
import { DiffFileSection } from "./DiffFileSection";
import { api } from "../../lib/wails";

interface DiffViewerProps {
  sessionId: string;
}

export function DiffViewer({ sessionId }: DiffViewerProps) {
  const diffFiles = useSessionStore((s) => s.diffFiles);
  const diffLoading = useSessionStore((s) => s.diffLoading);
  const fetchDiffFiles = useSessionStore((s) => s.fetchDiffFiles);
  const openEditorFile = useSessionStore((s) => s.openEditorFile);
  const intervalRef = useRef<ReturnType<typeof setInterval>>();

  // Initial fetch
  useEffect(() => {
    fetchDiffFiles(sessionId);
  }, [sessionId, fetchDiffFiles]);

  // Auto-refresh every 5 seconds
  useEffect(() => {
    intervalRef.current = setInterval(() => {
      fetchDiffFiles(sessionId);
    }, 5000);
    return () => clearInterval(intervalRef.current);
  }, [sessionId, fetchDiffFiles]);

  const handleRefresh = () => {
    fetchDiffFiles(sessionId);
  };

  const handleOpenFile = useCallback(
    async (filePath: string) => {
      try {
        const contents = await api().ReadFile(sessionId, filePath);
        openEditorFile(filePath, contents, detectLanguage(filePath));
      } catch (err) {
        console.error("Failed to open file from diff:", err);
      }
    },
    [sessionId, openEditorFile]
  );

  // Group files by submodule
  const rootFiles = diffFiles.filter((f) => !f.submodule);
  const submoduleGroups = new Map<string, typeof diffFiles>();
  for (const f of diffFiles) {
    if (f.submodule) {
      const group = submoduleGroups.get(f.submodule) ?? [];
      group.push(f);
      submoduleGroups.set(f.submodule, group);
    }
  }
  const hasSubmodules = submoduleGroups.size > 0;

  return (
    <div style={{ display: "flex", flexDirection: "column", height: "100%" }}>
      {/* Header */}
      <div
        style={{
          padding: "8px 12px",
          display: "flex",
          alignItems: "center",
          gap: 10,
          borderBottom: "1px solid var(--surface0)",
          background: "var(--base)",
          fontSize: 12,
        }}
      >
        <span style={{ color: "var(--text)", fontWeight: 600 }}>Changes</span>
        <span style={{ color: "var(--overlay0)" }}>
          {diffFiles.length} file{diffFiles.length !== 1 ? "s" : ""}
        </span>
        <span
          onClick={handleRefresh}
          style={{
            cursor: "pointer",
            color: "var(--overlay0)",
            fontSize: 13,
            marginLeft: "auto",
          }}
          title="Refresh diff"
        >
          {"\u21BB"}
        </span>
        {diffLoading && (
          <span style={{ color: "var(--overlay0)", fontSize: 10 }}>
            refreshing...
          </span>
        )}
      </div>

      {/* Diff content */}
      <div style={{ flex: 1, overflowY: "auto", background: "var(--mantle)" }}>
        {diffFiles.length === 0 && !diffLoading && (
          <div
            style={{
              display: "flex",
              alignItems: "center",
              justifyContent: "center",
              height: "100%",
              color: "var(--overlay0)",
              fontSize: 13,
            }}
          >
            No changes
          </div>
        )}

        {/* Root repo files */}
        {hasSubmodules && rootFiles.length > 0 && (
          <div
            style={{
              padding: "6px 12px",
              fontSize: 11,
              color: "var(--subtext0)",
              textTransform: "uppercase",
              background: "var(--base)",
              borderBottom: "1px solid var(--surface0)",
            }}
          >
            Root
          </div>
        )}
        {rootFiles.map((file) => (
          <DiffFileSection
            key={file.path}
            file={file}
            onOpenFile={handleOpenFile}
          />
        ))}

        {/* Submodule groups */}
        {Array.from(submoduleGroups.entries()).map(([smName, smFiles]) => (
          <div key={smName}>
            <div
              style={{
                padding: "6px 12px",
                fontSize: 11,
                color: "var(--subtext0)",
                textTransform: "uppercase",
                background: "var(--base)",
                borderBottom: "1px solid var(--surface0)",
              }}
            >
              {smName}
            </div>
            {smFiles.map((file) => (
              <DiffFileSection
                key={file.path}
                file={file}
                onOpenFile={handleOpenFile}
              />
            ))}
          </div>
        ))}
      </div>
    </div>
  );
}
