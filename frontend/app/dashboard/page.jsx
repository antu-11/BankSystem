'use client';

import { useState, useEffect, useCallback } from 'react';
import { motion } from 'framer-motion';
import { Send, RefreshCw, Plus } from 'lucide-react';
import BalanceCard from '@/components/BalanceCard';
import ActivityFeed from '@/components/ActivityFeed';
import TransferModal from '@/components/TransferModal';
import { accountsAPI } from '@/lib/api';

export default function DashboardPage() {
  const [accounts, setAccounts] = useState([]);
  const [activeAccount, setActiveAccount] = useState(null);
  const [balance, setBalance] = useState(null);
  const [history, setHistory] = useState([]);
  const [loading, setLoading] = useState(true);
  const [historyLoading, setHistoryLoading] = useState(true);
  const [transferOpen, setTransferOpen] = useState(false);

  const fetchData = useCallback(async () => {
    try {
      setLoading(true);
      const accountsData = await accountsAPI.getAll();
      setAccounts(accountsData.accounts || []);

      if (accountsData.accounts?.length > 0) {
        const acct = accountsData.accounts[0];
        setActiveAccount(acct);

        const [balanceData, historyData] = await Promise.all([
          accountsAPI.getBalance(acct.id),
          accountsAPI.getHistory(acct.id, 1, 20),
        ]);

        setBalance(balanceData);
        setHistory(historyData.items || []);
      }
    } catch (err) {
      console.error('Failed to load dashboard:', err);
    } finally {
      setLoading(false);
      setHistoryLoading(false);
    }
  }, []);

  useEffect(() => {
    fetchData();
  }, [fetchData]);

  const handleTransferSuccess = () => {
    fetchData(); // refresh all data
  };

  if (loading) {
    return <DashboardSkeleton />;
  }

  return (
    <div className="space-y-8">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <motion.h1
            initial={{ opacity: 0, y: -10 }}
            animate={{ opacity: 1, y: 0 }}
            className="text-2xl font-bold text-white"
          >
            Dashboard
          </motion.h1>
          <p className="text-slate-500 text-sm mt-0.5">
            Welcome back. Here&apos;s your financial overview.
          </p>
        </div>

        <div className="flex items-center gap-3">
          <motion.button
            whileHover={{ scale: 1.05 }}
            whileTap={{ scale: 0.95 }}
            onClick={fetchData}
            className="p-2.5 rounded-xl bg-white/5 border border-white/10 text-slate-400 hover:text-white hover:bg-white/10 transition-all cursor-pointer"
          >
            <RefreshCw className="w-4 h-4" />
          </motion.button>

          <motion.button
            whileHover={{ scale: 1.02 }}
            whileTap={{ scale: 0.98 }}
            onClick={() => setTransferOpen(true)}
            className="flex items-center gap-2 px-5 py-2.5 rounded-xl font-semibold text-white
                       bg-linear-to-r from-emerald-600 to-emerald-500
                       hover:from-emerald-500 hover:to-emerald-400
                       shadow-lg shadow-emerald-500/20 transition-all cursor-pointer"
          >
            <Send className="w-4 h-4" />
            Transfer
          </motion.button>
        </div>
      </div>

      {/* Balance + Quick Stats */}
      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        <div className="lg:col-span-2">
          <BalanceCard
            balance={balance?.balance}
            currency={balance?.currency}
            accountId={activeAccount?.id}
          />
        </div>

        <div className="space-y-4">
          <QuickStat
            label="Total Accounts"
            value={accounts.length}
            color="emerald"
          />
          <QuickStat
            label="Transactions"
            value={history.length}
            color="slate"
          />
          <QuickStat
            label="Account Status"
            value={activeAccount?.status || 'N/A'}
            color={activeAccount?.status === 'Active' ? 'emerald' : 'rose'}
            isText
          />
        </div>
      </div>

      {/* Account Selector (if multiple) */}
      {accounts.length > 1 && (
        <div className="glass-card p-4">
          <p className="text-sm text-slate-400 mb-3 font-medium">Your Accounts</p>
          <div className="flex gap-3 overflow-x-auto pb-1">
            {accounts.map((acct) => (
              <button
                key={acct.id}
                onClick={async () => {
                  setActiveAccount(acct);
                  const [b, h] = await Promise.all([
                    accountsAPI.getBalance(acct.id),
                    accountsAPI.getHistory(acct.id),
                  ]);
                  setBalance(b);
                  setHistory(h.items || []);
                }}
                className={`px-4 py-2 rounded-xl text-sm font-mono whitespace-nowrap transition-all cursor-pointer ${
                  activeAccount?.id === acct.id
                    ? 'bg-emerald-500/10 border border-emerald-500/20 text-emerald-400'
                    : 'bg-white/5 border border-white/10 text-slate-400 hover:bg-white/10'
                }`}
              >
                {acct.id.substring(0, 8)}... • {acct.currency}
              </button>
            ))}
          </div>
        </div>
      )}

      {/* Activity Feed */}
      <ActivityFeed transactions={history} loading={historyLoading} />

      {/* Transfer Modal */}
      <TransferModal
        isOpen={transferOpen}
        onClose={() => setTransferOpen(false)}
        fromAccountId={activeAccount?.id}
        onSuccess={handleTransferSuccess}
      />
    </div>
  );
}

// ── Quick Stat Card ──────────────────────────────────────────────
function QuickStat({ label, value, color = 'slate', isText = false }) {
  return (
    <motion.div
      initial={{ opacity: 0, x: 20 }}
      animate={{ opacity: 1, x: 0 }}
      className="glass-card p-4"
    >
      <p className="text-xs text-slate-500 font-medium">{label}</p>
      <p className={`text-2xl font-bold mt-1 ${
        color === 'emerald' ? 'text-emerald-400' :
        color === 'rose' ? 'text-rose-400' :
        'text-white'
      }`}>
        {isText ? value : value.toLocaleString()}
      </p>
    </motion.div>
  );
}

// ── Skeleton Loader ──────────────────────────────────────────────
function DashboardSkeleton() {
  return (
    <div className="space-y-8 animate-pulse">
      <div className="flex items-center justify-between">
        <div>
          <div className="h-7 w-40 bg-white/5 rounded-lg" />
          <div className="h-4 w-64 bg-white/5 rounded-lg mt-2" />
        </div>
        <div className="h-10 w-28 bg-white/5 rounded-xl" />
      </div>
      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        <div className="lg:col-span-2 h-48 bg-white/5 rounded-2xl" />
        <div className="space-y-4">
          <div className="h-20 bg-white/5 rounded-2xl" />
          <div className="h-20 bg-white/5 rounded-2xl" />
          <div className="h-20 bg-white/5 rounded-2xl" />
        </div>
      </div>
      <div className="h-96 bg-white/5 rounded-2xl" />
    </div>
  );
}
