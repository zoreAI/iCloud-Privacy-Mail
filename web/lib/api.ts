export class APIError extends Error { constructor(message: string, public status: number) { super(message); } }
export async function api<T>(path: string, options?: RequestInit): Promise<T> {
  const response = await fetch(path, { ...options, credentials: 'same-origin', headers: {'Content-Type':'application/json', ...options?.headers}, cache:'no-store' });
  const data = await response.json().catch(() => null);
  if (!response.ok || data?.success === false) throw new APIError(data?.message || `请求失败（HTTP ${response.status}）`, response.status);
  return data as T;
}
export const post = <T>(path: string, body: unknown = {}) => api<T>(path, {method:'POST', body:JSON.stringify(body)});
export type Mailbox = {id:string; email:string; label:string; status:string; created_at:string; api_url:string; account_label?:string; account_apple_id?:string; receive_count:number; api_active:boolean; icloud_active:boolean};
export type MailboxesResponse = {mailboxes:Mailbox[]; groups:{key:string;title:string;count:number}[]; pagination:{page:number;page_size:number;total:number;total_all:number;total_pages:number}};
export type User = {id?:string;username:string;is_admin:boolean;status?:string;created_at?:string;last_login_at?:string};
