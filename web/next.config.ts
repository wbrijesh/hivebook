import type { NextConfig } from "next"

const nextConfig: NextConfig = {
  // Self-contained build for the container image (.next/standalone + server.js).
  output: "standalone",
}

export default nextConfig
