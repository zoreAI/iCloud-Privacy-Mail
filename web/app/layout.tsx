import type { Metadata } from 'next';
import './globals.css';
import './forms.css';
export const metadata: Metadata = { title: 'Privacy Mail · 邮箱工作台', description: 'iCloud 隐私邮箱管理' };
export default function Layout({ children }: { children: React.ReactNode }) { return <html lang="zh-CN"><body>{children}</body></html>; }
