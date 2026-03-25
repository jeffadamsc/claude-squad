import { useSessionStore } from "../../store/sessionStore";

const statusColors: Record<string, string> = {
  running: "var(--green)",
  ready: "var(--yellow)",
  loading: "var(--subtext0)",
  paused: "var(--overlay0)",
};

export function ScopeSidebar() {
  const scopeMode = useSessionStore((s) => s.scopeMode);
  const sessions = useSessionStore((s) => s.sessions);
  const statuses = useSessionStore((s) => s.statuses);
  const exitScopeMode = useSessionStore((s) => s.exitScopeMode);

  const session = sessions.find((s) => s.id === scopeMode.sessionId);
  const status = session ? statuses.get(session.id) : undefined;

  return (
    <div
      style={{
        width: 110,
        background: "var(--base)",
        borderRight: "1px solid var(--surface0)",
        display: "flex",
        flexDirection: "column",
        height: "100%",
      }}
    >
      <div
        style={{
          padding: "12px 8px",
          fontWeight: "bold",
          fontSize: 12,
          borderBottom: "1px solid var(--surface0)",
          textAlign: "center",
          color: "var(--subtext0)",
        }}
      >
        {"\uD83D\uDD2C"} Scope
      </div>

      <div style={{ flex: 1, padding: 8 }}>
        {session && (
          <div
            style={{
              padding: "6px 8px",
              background: "var(--surface0)",
              borderRadius: 4,
              borderLeft: "2px solid var(--blue)",
            }}
          >
            <div
              style={{
                display: "flex",
                alignItems: "center",
                gap: 6,
                fontSize: 11,
              }}
            >
              <span
                style={{
                  color:
                    statusColors[status?.status ?? session.status] ??
                    "var(--overlay0)",
                  fontSize: 8,
                }}
              >
                {"\u25CF"}
              </span>
              <span
                style={{
                  color: "var(--text)",
                  overflow: "hidden",
                  textOverflow: "ellipsis",
                  whiteSpace: "nowrap",
                }}
              >
                {session.title}
              </span>
            </div>
            <div
              style={{
                fontSize: 10,
                color: "var(--overlay0)",
                marginTop: 2,
                overflow: "hidden",
                textOverflow: "ellipsis",
                whiteSpace: "nowrap",
              }}
            >
              {status?.branch ?? session.branch}
            </div>
          </div>
        )}
      </div>

      <div style={{ padding: 8, borderTop: "1px solid var(--surface0)" }}>
        <button
          onClick={exitScopeMode}
          style={{
            width: "100%",
            padding: 6,
            background: "var(--surface0)",
            color: "var(--text)",
            border: "none",
            borderRadius: 4,
            cursor: "pointer",
            fontSize: 11,
          }}
        >
          Exit Scope
        </button>
      </div>
    </div>
  );
}
