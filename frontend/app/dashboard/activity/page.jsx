'use client';

import { useState, useEffect } from 'react';
import { motion } from 'framer-motion';
import { ChevronLeft, ChevronRight } from 'lucide-react';
import ActivityFeed from '@/components/ActivityFeed';
import { accountsAPI } from '@/lib/api';

export default function ActivityPage() {
  const [accounts, setAccounts] = useState([]);
  const [activeAccount, setActiveAccount] = useState(null);
  const [history, setHistory] = useState([]);
  const [pagination, setPagination] = useState({ page: 1, per_page: 20, total: 0, total_pages: 0 });
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    async function load() {
      try {
        const data = await accountsAPI.getAll();
        const accts = data.accounts || [];
        setAccounts(accts);
        if (accts.length > 0) {
          setActiveAccount(accts[0]);
          await loadHistory(accts[0].id, 1);
        }
      } catch (err) {
        console.error(err);
      } finally {
        setLoading(false);
      }
    }
    load();
  }, []);

  const loadHistory = async (accountId, page) => {
    setLoading(true);
    try {
      const data = await accountsAPI.getHistory(accountId, page, 20);
      setHistory(data.items || []);
      setPagination({
        page: data.page,
        per_page: data.per_page,
        total: data.total,
        total_pages: data.total_pages,
      });
    } catch (err) {
      console.error(err);
    } finally {
      setLoading(false);
    }
  };

  const handleAccountChange = async (acct) => {
    setActiveAccount(acct);
    await loadHistory(acct.id, 1);
  };

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-bold text-white">Activity History</h1>
        <p className="text-slate-500 text-sm mt-0.5">Full transaction history for your accounts</p>
      </div>

      {/* Account selector */}
      {accounts.length > 1 && (
        <div className="flex gap-3 overflow-x-auto pb-1">
          {accounts.map((acct) => (
            <button
              key={acct.id}
              onClick={() => handleAccountChange(acct)}
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
      )}

      {/* Feed */}
      <ActivityFeed transactions={history} loading={loading} />

      {/* Pagination */}
      {pagination.total_pages > 1 && (
        <div className="flex items-center justify-center gap-4">
          <motion.button
            whileHover={{ scale: 1.05 }}
            whileTap={{ scale: 0.95 }}
            disabled={pagination.page <= 1}
            onClick={() => loadHistory(activeAccount.id, pagination.page - 1)}
            className="p-2 rounded-xl bg-white/5 border border-white/10 text-slate-400
                       hover:text-white hover:bg-white/10 disabled:opacity-30
                       disabled:cursor-not-allowed transition-all cursor-pointer"
          >
            <ChevronLeft className="w-4 h-4" />
          </motion.button>

          <span className="text-sm text-slate-400">
            Page {pagination.page} of {pagination.total_pages}
          </span>

          <motion.button
            whileHover={{ scale: 1.05 }}
            whileTap={{ scale: 0.95 }}
            disabled={pagination.page >= pagination.total_pages}
            onClick={() => loadHistory(activeAccount.id, pagination.page + 1)}
            className="p-2 rounded-xl bg-white/5 border border-white/10 text-slate-400
                       hover:text-white hover:bg-white/10 disabled:opacity-30
                       disabled:cursor-not-allowed transition-all cursor-pointer"
          >
            <ChevronRight className="w-4 h-4" />
          </motion.button>
        </div>
      )}
    </div>
  );
}
