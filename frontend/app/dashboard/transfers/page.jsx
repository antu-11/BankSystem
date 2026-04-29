'use client';

import { useState, useEffect } from 'react';
import { motion } from 'framer-motion';
import { Send } from 'lucide-react';
import TransferModal from '@/components/TransferModal';
import { accountsAPI } from '@/lib/api';

export default function TransfersPage() {
  const [accounts, setAccounts] = useState([]);
  const [selectedAccount, setSelectedAccount] = useState(null);
  const [transferOpen, setTransferOpen] = useState(false);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    async function load() {
      try {
        const data = await accountsAPI.getAll();
        const accts = data.accounts || [];
        setAccounts(accts);
        if (accts.length > 0) setSelectedAccount(accts[0]);
      } catch (err) {
        console.error(err);
      } finally {
        setLoading(false);
      }
    }
    load();
  }, []);

  if (loading) {
    return (
      <div className="space-y-4 animate-pulse">
        <div className="h-7 w-40 bg-white/5 rounded-lg" />
        <div className="h-48 bg-white/5 rounded-2xl" />
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-bold text-white">Transfers</h1>
        <p className="text-slate-500 text-sm mt-0.5">Send money to other accounts</p>
      </div>

      {/* Select source account */}
      {accounts.length > 0 && (
        <div className="glass-card p-6">
          <p className="text-sm text-slate-400 font-medium mb-3">Select Source Account</p>
          <div className="flex flex-wrap gap-3">
            {accounts.filter(a => a.status === 'Active').map((acct) => (
              <button
                key={acct.id}
                onClick={() => setSelectedAccount(acct)}
                className={`px-4 py-3 rounded-xl text-sm font-mono transition-all cursor-pointer ${
                  selectedAccount?.id === acct.id
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

      {/* Send */}
      <motion.div
        initial={{ opacity: 0, y: 15 }}
        animate={{ opacity: 1, y: 0 }}
        className="glass-card p-8 text-center"
      >
        <div className="w-16 h-16 mx-auto rounded-2xl bg-emerald-500/10 border border-emerald-500/20 flex items-center justify-center mb-4">
          <Send className="w-8 h-8 text-emerald-400" />
        </div>
        <h2 className="text-xl font-bold text-white">Send Funds</h2>
        <p className="text-slate-500 text-sm mt-1 max-w-sm mx-auto">
          Transfer money securely to any account. Transactions are atomic and ledger-verified.
        </p>
        <motion.button
          whileHover={{ scale: 1.02 }}
          whileTap={{ scale: 0.98 }}
          onClick={() => setTransferOpen(true)}
          disabled={!selectedAccount}
          className="mt-6 px-8 py-3 rounded-xl font-semibold text-white
                     bg-linear-to-r from-emerald-600 to-emerald-500
                     hover:from-emerald-500 hover:to-emerald-400
                     disabled:opacity-50 disabled:cursor-not-allowed
                     shadow-lg shadow-emerald-500/20 transition-all cursor-pointer
                     inline-flex items-center gap-2"
        >
          <Send className="w-4 h-4" />
          New Transfer
        </motion.button>
      </motion.div>

      <TransferModal
        isOpen={transferOpen}
        onClose={() => setTransferOpen(false)}
        fromAccountId={selectedAccount?.id}
        onSuccess={() => setTransferOpen(false)}
      />
    </div>
  );
}
