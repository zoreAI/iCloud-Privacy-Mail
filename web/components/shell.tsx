'use client';
import Link from 'next/link';
import { usePathname } from 'next/navigation';
import { Mail, ShieldCheck, LayoutGrid, UsersRound, PlusCircle, Settings2, Inbox, LogOut, BarChart3 } from 'lucide-react';
import { post } from '@/lib/api';
const navigation = [{href:'/',label:'邮箱池',icon:Inbox},{href:'/accounts',label:'账号与登录态',icon:UsersRound},{href:'/create',label:'创建与定时任务',icon:PlusCircle},{href:'/manage',label:'管理中心',icon:BarChart3},{href:'/settings',label:'设置与数据',icon:Settings2}];
export function Shell({children}:{children:React.ReactNode}) {
  const pathname=usePathname();
  return <div className="workspace"><aside><Link href="/" className="brand"><span className="brand-icon"><Mail size={23}/></span><span>Privacy Mail<small>iCloud 邮箱工作台</small></span></Link><div className="nav-caption">工作空间</div><nav>{navigation.map(({href,label,icon:Icon})=><Link key={href} href={href} className={pathname===href?'active':''}><Icon size={18}/>{label}</Link>)}</nav><div className="sidebar-note"><ShieldCheck size={20}/><strong>清晰管理每一个地址</strong><p>邮箱地址与邮件内容分开管理，重要操作始终需要确认。</p></div></aside><section className="workspace-main"><header className="topbar"><span><LayoutGrid size={16}/> 工作台 <span className="slash">/</span> {navigation.find(n=>n.href===pathname)?.label||'管理'}</span><button className="quiet" onClick={async()=>{await post('/api/auth/logout');location.href='/login';}}><LogOut size={16}/>退出登录</button></header><main>{children}</main><footer>PRIVACY MAIL <span>让邮箱管理，回归有序。</span></footer></section></div>;
}
