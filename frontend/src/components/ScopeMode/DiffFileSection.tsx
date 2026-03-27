import { useState, useRef, useEffect } from "react";
import { DiffEditor } from "@monaco-editor/react";
import type { DiffFile } from "../../lib/wails";
import { detectLanguage } from "../../store/sessionStore";
import { catppuccinMocha } from "../../lib/monacoTheme";
import type * as Monaco from "monaco-editor";

interface DiffFileSectionProps {
  file: DiffFile;
  onOpenFile: (path: string) => void;
}

const statusColors: Record<string, string> = {
  added: "var(--green)",
  modified: "var(--yellow)",
  deleted: "var(--red)",
};

const statusLabels: Record<string, string> = {
  added: "A",
  modified: "M",
  deleted: "D",
};

export function DiffFileSection({ file, onOpenFile }: DiffFileSectionProps) {
  const [collapsed, setCollapsed] = useState(false);
  const [visible, setVisible] = useState(false);
  const containerRef = useRef<HTMLDivElement>(null);

  // Lazy-render: only mount DiffEditor when near viewport
  useEffect(() => {
    const el = containerRef.current;
    if (!el) return;
    const observer = new IntersectionObserver(
      ([entry]) => {
        if (entry.isIntersecting) {
          setVisible(true);
          observer.disconnect();
        }
      },
      { rootMargin: "200px" }
    );
    observer.observe(el);
    return () => observer.disconnect();
  }, []);

  const language = detectLanguage(file.path);

  // Count lines to set a reasonable editor height
  const maxLines = Math.max(
    (file.oldContent || "").split("\n").length,
    (file.newContent || "").split("\n").length
  );
  const editorHeight = Math.min(Math.max(maxLines * 19 + 20, 80), 600);

  const handleMount = (
    _editor: Monaco.editor.IStandaloneDiffEditor,
    monaco: typeof Monaco
  ) => {
    monaco.editor.defineTheme("catppuccin-mocha", catppuccinMocha);
    monaco.editor.setTheme("catppuccin-mocha");
  };

  return (
    <div ref={containerRef} style={{ marginBottom: 2 }}>
      {/* File header */}
      <div
        style={{
          display: "flex",
          alignItems: "center",
          gap: 8,
          padding: "6px 12px",
          background: "var(--surface0)",
          cursor: "pointer",
          userSelect: "none",
          fontSize: 12,
        }}
        onClick={() => setCollapsed(!collapsed)}
      >
        <span style={{ color: "var(--overlay0)", fontSize: 10 }}>
          {collapsed ? "\u25B6" : "\u25BC"}
        </span>
        <span
          style={{
            color: statusColors[file.status] ?? "var(--text)",
            fontWeight: 600,
            fontSize: 11,
            minWidth: 14,
          }}
        >
          {statusLabels[file.status] ?? "?"}
        </span>
        <span
          onClick={(e) => {
            e.stopPropagation();
            onOpenFile(file.path);
          }}
          style={{
            color: "var(--blue)",
            cursor: "pointer",
            textDecoration: "underline",
            fontFamily: "'JetBrains Mono', 'Fira Code', monospace",
          }}
          title={`Open ${file.path}`}
        >
          {file.path}
        </span>
        {file.submodule && (
          <span style={{ color: "var(--overlay0)", fontSize: 10 }}>
            ({file.submodule})
          </span>
        )}
      </div>

      {/* Diff editor */}
      {!collapsed && (
        <div
          style={{
            height: editorHeight,
            borderBottom: "1px solid var(--surface0)",
          }}
        >
          {visible ? (
            <DiffEditor
              original={file.oldContent}
              modified={file.newContent}
              language={language}
              theme="catppuccin-mocha"
              onMount={handleMount}
              options={{
                readOnly: true,
                renderSideBySide: false,
                minimap: { enabled: false },
                fontSize: 13,
                fontFamily:
                  "'JetBrains Mono', 'Fira Code', 'Cascadia Code', monospace",
                scrollBeyondLastLine: false,
                automaticLayout: true,
                lineNumbers: "on",
                renderOverviewRuler: false,
                padding: { top: 4 },
              }}
            />
          ) : (
            <div
              style={{
                height: "100%",
                display: "flex",
                alignItems: "center",
                justifyContent: "center",
                color: "var(--overlay0)",
                fontSize: 12,
              }}
            >
              Loading...
            </div>
          )}
        </div>
      )}
    </div>
  );
}
