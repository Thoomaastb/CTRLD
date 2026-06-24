import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  // Standalone-Output für Docker
  output: "standalone",

  async rewrites() {
    return [
      {
        source: "/api/:path*",
        destination: `${process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8443/api"}/:path*`,
      },
    ];
  },
};

export default nextConfig;
