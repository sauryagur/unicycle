"use client";
import { useEffect } from "react";
export function ServiceWorkerRegistration() { useEffect(() => { if ("serviceWorker" in navigator) void navigator.serviceWorker.register("/sw.js").catch(() => undefined); }, []); return null; }
