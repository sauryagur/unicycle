import type { Metadata, Viewport } from "next";
import "./globals.css";
import { Providers } from "@/components/providers";
import { ServiceWorkerRegistration } from "@/components/sw-registration";
export const metadata: Metadata = { title: "Unicycle — Campus rides, made simple", description: "Find a bicycle, start your campus ride, and manage your wallet.", applicationName: "Unicycle", appleWebApp: { capable: true, statusBarStyle: "default", title: "Unicycle" } };
export const viewport: Viewport = { themeColor: "#f6f8f5", width: "device-width", initialScale: 1 };
export default function RootLayout({ children }: Readonly<{ children: React.ReactNode }>) { return <html lang="en"><body><Providers>{children}</Providers><ServiceWorkerRegistration /></body></html>; }
