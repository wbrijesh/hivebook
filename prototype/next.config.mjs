/** @type {import('next').NextConfig} */
const nextConfig = {
  devIndicators: false,
  // Self-contained build for the container image (.next/standalone + server.js).
  output: 'standalone',
}

export default nextConfig
