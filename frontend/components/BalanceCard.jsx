'use client';

import { motion } from 'framer-motion';
import { TrendingUp, Wallet } from 'lucide-react';

export default function BalanceCard({ balance, currency = 'INR', accountId }) {
  // Format to Indian currency
  const formatted = new Intl.NumberFormat('en-IN', {
    style: 'currency',
    currency: currency,
    minimumFractionDigits: 2,
    maximumFractionDigits: 2,
  }).format(parseFloat(balance || 0));

  return (
    <motion.div
      initial={{ opacity: 0, y: 20 }}
      animate={{ opacity: 1, y: 0 }}
      transition={{ duration: 0.5 }}
      className="relative overflow-hidden rounded-2xl"
    >
      {/* Gradient background */}
      <div className="absolute inset-0 bg-linear-to-br from-emerald-600/20 via-emerald-500/10 to-transparent" />
      <div className="absolute -top-24 -right-24 w-64 h-64 bg-emerald-500/10 rounded-full blur-3xl" />
      <div className="absolute -bottom-16 -left-16 w-48 h-48 bg-emerald-600/10 rounded-full blur-2xl" />

      <div className="relative glass-card p-8">
        <div className="flex items-start justify-between mb-6">
          <div>
            <p className="text-sm text-slate-400 font-medium flex items-center gap-2">
              <Wallet className="w-4 h-4" />
              Total Balance
            </p>
            {accountId && (
              <p className="text-xs text-slate-600 mt-1 font-mono">
                {accountId.substring(0, 8)}...{accountId.substring(accountId.length - 4)}
              </p>
            )}
          </div>
          <div className="flex items-center gap-1.5 px-3 py-1 rounded-full bg-emerald-500/10 border border-emerald-500/20">
            <TrendingUp className="w-3.5 h-3.5 text-emerald-400" />
            <span className="text-xs font-medium text-emerald-400">Active</span>
          </div>
        </div>

        <motion.div
          initial={{ opacity: 0, scale: 0.9 }}
          animate={{ opacity: 1, scale: 1 }}
          transition={{ delay: 0.2, duration: 0.4 }}
        >
          <p className="text-4xl md:text-5xl font-bold text-white tracking-tight">
            {formatted}
          </p>
          <p className="text-sm text-slate-500 mt-2">{currency} • Updated just now</p>
        </motion.div>

        {/* Decorative line */}
        <div className="mt-6 h-px bg-linear-to-r from-transparent via-emerald-500/30 to-transparent" />

        <div className="mt-4 flex items-center gap-4">
          <div className="flex items-center gap-2">
            <div className="w-2 h-2 rounded-full bg-emerald-400" />
            <span className="text-xs text-slate-400">Ledger-verified balance</span>
          </div>
        </div>
      </div>
    </motion.div>
  );
}
