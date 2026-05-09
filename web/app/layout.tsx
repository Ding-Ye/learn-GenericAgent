import type { Metadata, Viewport } from "next";
import type { ReactNode } from "react";

export const metadata: Metadata = {
  title: "learn-GenericAgent",
  description: "10-chapter Go re-implementation of lsdefine/GenericAgent",
};

export const viewport: Viewport = {
  width: "device-width",
  initialScale: 1,
};

export default function RootLayout({ children }: { children: ReactNode }) {
  return (
    <html lang="en">
      <body
        style={{
          fontFamily:
            "ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif",
          margin: 0,
          padding: 0,
          background: "#0b0c10",
          color: "#e6edf3",
        }}
      >
        {children}
      </body>
    </html>
  );
}
