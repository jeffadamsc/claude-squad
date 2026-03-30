import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";

interface MarkdownPreviewProps {
  content: string;
  fontSize: number;
}

export function MarkdownPreview({ content, fontSize }: MarkdownPreviewProps) {
  return (
    <div
      style={{
        height: "100%",
        overflow: "auto",
        padding: "16px 24px",
        background: "var(--base)",
        color: "var(--text)",
        fontFamily:
          "-apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif",
        fontSize,
        lineHeight: 1.6,
      }}
    >
      <div className="markdown-preview">
        <ReactMarkdown remarkPlugins={[remarkGfm]}>{content}</ReactMarkdown>
      </div>
      <style>{`
        .markdown-preview h1,
        .markdown-preview h2,
        .markdown-preview h3,
        .markdown-preview h4,
        .markdown-preview h5,
        .markdown-preview h6 {
          color: var(--text);
          margin-top: 1.2em;
          margin-bottom: 0.4em;
          font-weight: 600;
        }
        .markdown-preview h1 { font-size: 1.8em; border-bottom: 1px solid var(--surface0); padding-bottom: 0.3em; }
        .markdown-preview h2 { font-size: 1.4em; border-bottom: 1px solid var(--surface0); padding-bottom: 0.3em; }
        .markdown-preview h3 { font-size: 1.2em; }
        .markdown-preview p { margin: 0.6em 0; }
        .markdown-preview a { color: var(--blue); text-decoration: none; }
        .markdown-preview a:hover { text-decoration: underline; }
        .markdown-preview code {
          background: var(--surface0);
          padding: 2px 5px;
          border-radius: 3px;
          font-family: 'JetBrains Mono', 'Fira Code', monospace;
          font-size: 0.9em;
        }
        .markdown-preview pre {
          background: var(--mantle);
          padding: 12px 16px;
          border-radius: 6px;
          overflow-x: auto;
          margin: 0.8em 0;
        }
        .markdown-preview pre code {
          background: none;
          padding: 0;
        }
        .markdown-preview blockquote {
          border-left: 3px solid var(--blue);
          margin: 0.8em 0;
          padding: 4px 16px;
          color: var(--subtext0);
        }
        .markdown-preview ul, .markdown-preview ol {
          padding-left: 1.5em;
          margin: 0.4em 0;
        }
        .markdown-preview li { margin: 0.2em 0; }
        .markdown-preview table {
          border-collapse: collapse;
          margin: 0.8em 0;
          width: 100%;
        }
        .markdown-preview th, .markdown-preview td {
          border: 1px solid var(--surface1);
          padding: 6px 12px;
          text-align: left;
        }
        .markdown-preview th {
          background: var(--surface0);
          font-weight: 600;
        }
        .markdown-preview hr {
          border: none;
          border-top: 1px solid var(--surface0);
          margin: 1.2em 0;
        }
        .markdown-preview img {
          max-width: 100%;
        }
        .markdown-preview input[type="checkbox"] {
          margin-right: 6px;
        }
      `}</style>
    </div>
  );
}
