import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  // Strikte React-Features aktivieren
  reactStrictMode: true,

  // API-Proxy: alle /api/* Requests an das Go-Backend weiterleiten
  async rewrites() {
    const backendUrl = process.env.CTRLD_API_URL ?? "https://localhost:8443";
    return [
      {
        source: "/api/:path*",
        destination: `${backendUrl}/api/:path*`,
      },
    ];
  },

  // Security-Header auch für Next.js (zusätzlich zu Go)
  async headers() {
    return [
      {
        source: "/(.*)",
        headers: [
          { key: "X-Frame-Options", value: "DENY" },
          { key: "X-Content-Type-Options", value: "nosniff" },
          { key: "Referrer-Policy", value: "strict-origin-when-cross-origin" },
          { key: "Permissions-Policy", value: "geolocation=(), microphone=(), camera=()" },
        ],
      },
    ];
  },
};

export default nextConfig;
