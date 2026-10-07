import type { NextConfig } from "next";

const apiOrigin = process.env.API_ORIGIN ?? "http://localhost:8080";
const nextConfig: NextConfig = { async rewrites() { return [{ source: "/api/backend/:path*", destination: `${apiOrigin}/v1/:path*` }]; } };
export default nextConfig;
