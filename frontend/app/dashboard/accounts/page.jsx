'use client';

import { useState, useEffect } from 'react';
import { motion } from 'framer-motion';
import { Wallet, Copy, Check } from 'lucide-react';
import { accountsAPI } from '@/lib/api';

export default function AccountsPage() {
  const [accounts, setAccounts] = useState([]);
  const [balances, setBalances] = useState({});
  const [loading, setLoading] = useState(true);
  const [copiedId, setCopiedId] = useState(null);

  useEffect(() => {
    async function load() {
      try {
        const data = await accountsAPI.getAll();
        const accts = data.accounts || [];
        setAccounts(accts);

        // Fetch balances in parallel
        const balanceEntries = await Promise.all(
          accts.map(async (a) => {
            try {
              const b = await accountsAPI.getBalance(a.id);
              return [a.id, b.balance];
            } catch {
              return [a.id, '0.0000'];
            }
          })
        );
        setBalances(Object.fromEntries(balanceEntries));
      } catch (err) {
        console.error(err);
      } finally {
        setLoading(false);
      }
    }
    load();
  }, []);

  const copyId = (id) => {
    navigator.clipboard.writeText(id);
    setCopiedId(id);
    setTimeout(() => setCopiedId(null), 2000);
  };

  if (loading) {
    return (
      <div className="space-y-4 animate-pulse">
        <div className="h-7 w-40 bg-white/5 rounded-lg" />
        {[1, 2].map((i) => (
          <div key={i} className="h-32 bg-white/5 rounded-2xl" />
        ))}
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-bold text-white">Your Accounts</h1>
        <p className="text-slate-500 text-sm mt-0.5">Manage and view all your banking accounts</p>
      </div>

      {accounts.length === 0 ? (
        <div className="glass-card p-12 text-center">
          <Wallet className="w-12 h-12 text-slate-600 mx-auto mb-4" />
          <p className="text-slate-400">No accounts found</p>
        </div>
      ) : (
        <div className="grid gap-4">
          {accounts.map((acct, i) => (
            <motion.div
              key={acct.id}
              initial={{ opacity: 0, y: 15 }}
              animate={{ opacity: 1, y: 0 }}
              transition={{ delay: i * 0.1 }}
              className="glass-card p-6"
            >
              <div className="flex items-start justify-between">
                <div className="flex items-center gap-3">
                  <div className={`w-10 h-10 rounded-xl flex items-center justify-center ${
                    acct.status === 'Active'
                      ? 'bg-emerald-500/10 text-emerald-400'
                      : acct.status === 'Frozen'
                      ? 'bg-blue-500/10 text-blue-400'
                      : 'bg-slate-500/10 text-slate-400'
                  }`}>
                    <Wallet className="w-5 h-5" />
                  </div>
                  <div>
                    <p className="text-white font-semibold">{acct.currency} Account</p>
                    <div className="flex items-center gap-2 mt-0.5">
                      <p className="text-xs text-slate-500 font-mono">{acct.id}</p>
                      <button
                        onClick={() => copyId(acct.id)}
                        className="text-slate-600 hover:text-emerald-400 transition-colors cursor-pointer"
                      >
                        {copiedId === acct.id ? (
                          <Check className="w-3 h-3" />
                        ) : (
                          <Copy className="w-3 h-3" />
                        )}
                      </button>
                    </div>
                  </div>
                </div>

                <div className={`px-2.5 py-1 rounded-full text-xs font-medium border ${
                  acct.status === 'Active'
                    ? 'bg-emerald-500/20 text-emerald-400 border-emerald-500/30'
                    : acct.status === 'Frozen'
                    ? 'bg-blue-500/20 text-blue-400 border-blue-500/30'
                    : 'bg-slate-500/20 text-slate-400 border-slate-500/30'
                }`}>
                  {acct.status}
                </div>
              </div>

              <div className="mt-4 pt-4 border-t border-white/5">
                <p className="text-xs text-slate-500">Balance</p>
                <p className="text-2xl font-bold text-white mt-1">
                  {new Intl.NumberFormat('en-IN', {
                    style: 'currency',
                    currency: acct.currency,
                  }).format(parseFloat(balances[acct.id] || 0))}
                </p>
              </div>

              <div className="mt-3 flex items-center gap-2">
                <p className="text-xs text-slate-600">
                  Created {new Date(acct.created_at).toLocaleDateString('en-IN', {
                    day: 'numeric', month: 'short', year: 'numeric'
                  })}
                </p>
              </div>
            </motion.div>
          ))}
        </div>
      )}
    </div>
  );
}
