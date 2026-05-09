import Link from "next/link";
import { notFound } from "next/navigation";
import { SESSIONS } from "../../../lib/curriculum";
import { promises as fs } from "node:fs";
import path from "node:path";

export async function generateStaticParams() {
  return SESSIONS.filter((s) => s.available).map((s) => ({ slug: s.slug }));
}

async function loadDoc(slug: string, lang: "zh" | "en"): Promise<string | null> {
  try {
    const p = path.join(process.cwd(), "..", "docs", lang, `${slug}.md`);
    return await fs.readFile(p, "utf-8");
  } catch {
    return null;
  }
}

export default async function SessionPage({
  params,
}: {
  params: Promise<{ slug: string }>;
}) {
  const { slug } = await params;
  const session = SESSIONS.find((s) => s.slug === slug);
  if (!session || !session.available) notFound();

  const zh = await loadDoc(slug, "zh");
  const en = await loadDoc(slug, "en");

  return (
    <main style={{ maxWidth: 880, margin: "0 auto", padding: "3rem 1.5rem" }}>
      <p>
        <Link href="/" style={{ color: "#7dd3fc" }}>
          ← back
        </Link>
      </p>
      <h1>
        {session.num} · {session.titleZh}
      </h1>
      <p style={{ color: "#9aa6b2" }}>
        {session.titleEn} · upstream: <code>{session.upstream}</code>
      </p>

      <h2 style={{ marginTop: "2rem" }}>中文</h2>
      <pre
        style={{
          whiteSpace: "pre-wrap",
          background: "#111827",
          padding: "1rem",
          borderRadius: 8,
          fontSize: "0.85rem",
          lineHeight: 1.6,
        }}
      >
        {zh ?? "doc not found"}
      </pre>

      <h2 style={{ marginTop: "2rem" }}>English</h2>
      <pre
        style={{
          whiteSpace: "pre-wrap",
          background: "#111827",
          padding: "1rem",
          borderRadius: 8,
          fontSize: "0.85rem",
          lineHeight: 1.6,
        }}
      >
        {en ?? "doc not found"}
      </pre>
    </main>
  );
}
