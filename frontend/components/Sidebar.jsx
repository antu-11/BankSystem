'use client';

import { useRouter, usePathname } from 'next/navigation';
import { motion } from 'framer-motion';
import {
  LayoutDashboard,
  ArrowLeftRight,
  History,
  LogOut,
  Shield,
  Wallet,
} from 'lucide-react';
import { authAPI } from '@/lib/api';

const navItems = [
  { label: 'Overview', href: '/dashboard', icon: LayoutDashboard },
  { label: 'Accounts', href: '/dashboard/accounts', icon: Wallet },
  { label: 'Transfers', href: '/dashboard/transfers', icon: ArrowLeftRight },
  { label: 'Activity', href: '/dashboard/activity', icon: History },
];

export default function Sidebar() {
  const router = useRouter();
  const pathname = usePathname();

  const handleLogout = async () => {
    try {
      await authAPI.logout();
    } catch {
      // ignore — cookie may already be expired
    }
    router.push('/login');
  };

  return (
    <aside className="w-64 h-screen sticky top-0 flex flex-col glass border-r border-white/5">
      {/* Brand */}
      <div className="p-6 flex items-center gap-3">
        <div className="w-10 h-10 rounded-xl bg-emerald-500/10 border border-emerald-500/20 flex items-center justify-center">
          <Shield className="w-5 h-5 text-emerald-400" />
        </div>
        <div>
          <h2 className="text-lg font-bold text-white tracking-tight">The Vault</h2>
          <p className="text-xs text-slate-500">Banking System</p>
        </div>
      </div>

      {/* Navigation */}
      <nav className="flex-1 px-3 mt-4 space-y-1">
        {navItems.map(({ label, href, icon: Icon }) => {
          const isActive = pathname === href;
          return (
            <motion.button
              key={href}
              whileHover={{ x: 4 }}
              whileTap={{ scale: 0.98 }}
              onClick={() => router.push(href)}
              className={`w-full flex items-center gap-3 px-4 py-2.5 rounded-xl text-sm font-medium transition-all duration-200 cursor-pointer ${
                isActive
                  ? 'bg-emerald-500/10 text-emerald-400 border border-emerald-500/20'
                  : 'text-slate-400 hover:text-slate-200 hover:bg-white/5'
              }`}
            >
              <Icon className="w-4 h-4" />
              {label}
              {isActive && (
                <motion.div
                  layoutId="activeIndicator"
                  className="ml-auto w-1.5 h-1.5 rounded-full bg-emerald-400"
                />
              )}
            </motion.button>
          );
        })}
      </nav>

      {/* Logout */}
      <div className="p-3 border-t border-white/5">
        <motion.button
          whileHover={{ x: 4 }}
          whileTap={{ scale: 0.98 }}
          onClick={handleLogout}
          className="w-full flex items-center gap-3 px-4 py-2.5 rounded-xl text-sm font-medium text-rose-400 hover:bg-rose-500/10 transition-all duration-200 cursor-pointer"
        >
          <LogOut className="w-4 h-4" />
          Sign Out
        </motion.button>
      </div>
    </aside>
  );
}
