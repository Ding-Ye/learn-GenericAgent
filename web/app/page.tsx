import Link from "next/link";
import { SESSIONS } from "../lib/curriculum";

export default function Home() {
  return (
    <main style={{ maxWidth: 880, margin: "0 auto", padding: "3rem 1.5rem" }}>
      <h1 style={{ fontSize: "2rem", marginBottom: "0.25rem" }}>
        learn-GenericAgent
      </h1>
      <p style={{ color: "#9aa6b2", marginTop: 0 }}>
        10-章 Go 学习实现 · companion to{" "}
        <a
          href="https://github.com/lsdefine/GenericAgent"
          style={{ color: "#7dd3fc" }}
        >
          lsdefine/GenericAgent
        </a>
      </p>

      <h2 style={{ marginTop: "2.5rem", fontSize: "1.25rem" }}>课程目录</h2>
      <ul style={{ listStyle: "none", padding: 0 }}>
        {SESSIONS.map((s) => (
          <li
            key={s.slug}
            style={{
              borderBottom: "1px solid #1f2937",
              padding: "0.75rem 0",
              display: "flex",
              gap: "1rem",
              alignItems: "baseline",
            }}
          >
            <span
              style={{
                color: s.available ? "#34d399" : "#52525b",
                width: 64,
                fontFamily: "ui-monospace, monospace",
                fontSize: "0.875rem",
              }}
            >
              {s.available ? "✅" : "⏳"} {s.num}
            </span>
            <div style={{ flex: 1 }}>
              {s.available ? (
                <Link
                  href={`/s/${s.slug}`}
                  style={{ color: "#e6edf3", textDecoration: "none" }}
                >
                  <strong>{s.titleZh}</strong>
                </Link>
              ) : (
                <strong style={{ color: "#9aa6b2" }}>{s.titleZh}</strong>
              )}
              <div style={{ color: "#9aa6b2", fontSize: "0.875rem" }}>
                {s.titleEn} · upstream: <code>{s.upstream}</code>
              </div>
            </div>
          </li>
        ))}
      </ul>

      <p style={{ color: "#71717a", fontSize: "0.875rem", marginTop: "3rem" }}>
        Source on{" "}
        <a
          href="https://github.com/Ding-Ye/learn-GenericAgent"
          style={{ color: "#7dd3fc" }}
        >
          GitHub
        </a>
        . Pedagogy adapted from{" "}
        <a
          href="https://github.com/shareAI-lab/learn-claude-code"
          style={{ color: "#7dd3fc" }}
        >
          shareAI-lab/learn-claude-code
        </a>
        .
      </p>
    </main>
  );
}
