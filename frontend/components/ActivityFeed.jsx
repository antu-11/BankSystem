'use client';

import { motion } from 'framer-motion';
import {
  ArrowUpRight,
  ArrowDownLeft,
  Clock,
  CheckCircle2,
  XCircle,
  RotateCcw,
} from 'lucide-react';

const statusConfig = {
  Completed: {
    badge: 'bg-emerald-500/20 text-emerald-400 border-emerald-500/30',
    icon: CheckCircle2,
    label: 'Completed',
  },
  Failed: {
    badge: 'bg-rose-500/20 text-rose-400 border-rose-500/30',
    icon: XCircle,
    label: 'Failed',
  },
  Pending: {
    badge: 'bg-amber-500/20 text-amber-400 border-amber-500/30',
    icon: Clock,
    label: 'Pending',
  },
  Reversed: {
    badge: 'bg-slate-500/20 text-slate-400 border-slate-500/30',
    icon: RotateCcw,
    label: 'Reversed',
  },
};

function formatCurrency(amount) {
  return new Intl.NumberFormat('en-IN', {
    style: 'currency',
    currency: 'INR',
    minimumFractionDigits: 2,
  }).format(parseFloat(amount));
}

function formatDate(dateStr) {
  const date = new Date(dateStr);
  const now = new Date();
  const diffMs = now - date;
  const diffMins = Math.floor(diffMs / 60000);
  const diffHours = Math.floor(diffMs / 3600000);
  const diffDays = Math.floor(diffMs / 86400000);

  if (diffMins < 1) return 'Just now';
  if (diffMins < 60) return `${diffMins}m ago`;
  if (diffHours < 24) return `${diffHours}h ago`;
  if (diffDays < 7) return `${diffDays}d ago`;

  return date.toLocaleDateString('en-IN', {
    day: 'numeric',
    month: 'short',
    year: date.getFullYear() !== now.getFullYear() ? 'numeric' : undefined,
  });
}

function TransactionItem({ txn, index }) {
  const isSent = txn.direction === 'sent';
  const config = statusConfig[txn.status] || statusConfig.Pending;
  const StatusIcon = config.icon;

  return (
    <motion.div
      initial={{ opacity: 0, y: 10 }}
      animate={{ opacity: 1, y: 0 }}
      transition={{ delay: index * 0.05 }}
      className="flex items-center gap-4 p-4 rounded-xl hover:bg-white/2 transition-colors group"
    >
      {/* Direction icon */}
      <div
        className={`w-10 h-10 rounded-xl flex items-center justify-center shrink-0 ${
          isSent
            ? 'bg-rose-500/10 text-rose-400'
            : 'bg-emerald-500/10 text-emerald-400'
        }`}
      >
        {isSent ? (
          <ArrowUpRight className="w-5 h-5" />
        ) : (
          <ArrowDownLeft className="w-5 h-5" />
        )}
      </div>

      {/* Details */}
      <div className="flex-1 min-w-0">
        <p className="text-sm font-medium text-white truncate">
          {isSent ? 'Sent' : 'Received'} Transfer
        </p>
        <p className="text-xs text-slate-500 font-mono truncate mt-0.5">
          {isSent ? txn.to_account_id : txn.from_account_id}
        </p>
      </div>

      {/* Amount */}
      <div className="text-right shrink-0">
        <p className={`text-sm font-semibold ${isSent ? 'text-rose-400' : 'text-emerald-400'}`}>
          {isSent ? '-' : '+'}{formatCurrency(txn.amount)}
        </p>
        <p className="text-xs text-slate-600 mt-0.5">{formatDate(txn.created_at)}</p>
      </div>

      {/* Status badge */}
      <div
        className={`px-2.5 py-1 rounded-full text-xs font-medium border shrink-0 ${config.badge}`}
      >
        <span className="flex items-center gap-1">
          <StatusIcon className="w-3 h-3" />
          {config.label}
        </span>
      </div>
    </motion.div>
  );
}

export default function ActivityFeed({ transactions = [], loading = false }) {
  if (loading) {
    return (
      <div className="glass-card p-6">
        <h3 className="text-lg font-bold text-white mb-4">Recent Activity</h3>
        <div className="space-y-3">
          {[...Array(5)].map((_, i) => (
            <div key={i} className="flex items-center gap-4 p-4 animate-pulse">
              <div className="w-10 h-10 rounded-xl bg-white/5" />
              <div className="flex-1 space-y-2">
                <div className="h-4 w-32 rounded bg-white/5" />
                <div className="h-3 w-48 rounded bg-white/5" />
              </div>
              <div className="h-4 w-20 rounded bg-white/5" />
            </div>
          ))}
        </div>
      </div>
    );
  }

  return (
    <div className="glass-card p-6">
      <div className="flex items-center justify-between mb-4">
        <h3 className="text-lg font-bold text-white">Recent Activity</h3>
        <span className="text-xs text-slate-500">
          {transactions.length} transaction{transactions.length !== 1 ? 's' : ''}
        </span>
      </div>

      {transactions.length === 0 ? (
        <div className="py-12 text-center">
          <Clock className="w-10 h-10 text-slate-600 mx-auto mb-3" />
          <p className="text-slate-500 text-sm">No transactions yet</p>
          <p className="text-slate-600 text-xs mt-1">Your activity will appear here</p>
        </div>
      ) : (
        <div className="space-y-1 max-h-[480px] overflow-y-auto pr-1">
          {transactions.map((txn, i) => (
            <TransactionItem key={txn.id} txn={txn} index={i} />
          ))}
        </div>
      )}
    </div>
  );
}
