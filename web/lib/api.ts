export const API_BASE = "/api/backend";
export type ApiError = Error & { status: number; code?: string; details?: unknown };
export function getToken() { return typeof window === "undefined" ? null : window.localStorage.getItem("unicycle_token"); }
export function setToken(token: string) { window.localStorage.setItem("unicycle_token", token); }
export function clearToken() { window.localStorage.removeItem("unicycle_token"); }
export async function api<T>(path: string, init: RequestInit = {}): Promise<T> { const token=getToken(); const headers=new Headers(init.headers); if(init.body&&!headers.has("Content-Type"))headers.set("Content-Type","application/json"); if(token)headers.set("Authorization","Bearer "+token); const response=await fetch(API_BASE+path,{...init,headers,cache:"no-store"}); if(response.status===204)return undefined as T; const payload=await response.json().catch(()=>null); if(!response.ok){const error=new Error(payload?.message??`Request failed (${response.status})`) as ApiError;error.status=response.status;error.code=payload?.code;error.details=payload?.details;throw error;}return payload as T;}
export function formatMoney(paise?:number|null){return new Intl.NumberFormat("en-IN",{style:"currency",currency:"INR"}).format((paise??0)/100)}
export function formatDuration(seconds?:number|null){if(seconds==null)return "In progress";const m=Math.floor(seconds/60);return m?`${m} min ${seconds%60} sec`:`${seconds} sec`}
export function apiErrorMessage(error:unknown){return error instanceof Error?error.message:"Something went wrong. Please try again."}
export async function currentRide():Promise<import("@/lib/types").Ride|null>{try{return await api("/rides/current")}catch(error){if((error as ApiError).status===404)return null;throw error}}
