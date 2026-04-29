'use client';

import { useState } from 'react';
import { motion, AnimatePresence } from 'framer-motion';
import { X, Send, CheckCircle2, AlertCircle, Loader2 } from 'lucide-react';
import { transfersAPI } from '@/lib/api';
import { v4 as uuidv4 } from 'uuid';

export default function TransferModal({ isOpen, onClose, fromAccountId, onSuccess }) {
  const [form, setForm] = useState({ to_account_id: '', amount: '' });
  const [status, setStatus] = useState('idle'); // idle | loading | success | error
  const [error, setError] = useState('');

  const handleSubmit = async (e) => {
    e.preventDefault();
    setStatus('loading');
    setError('');

    try {
      await transfersAPI.send({
        from_account_id: fromAccountId,
        to_account_id: form.to_account_id,
        amount: form.amount,
        idempotency_key: uuidv4(),
      });
      setStatus('success');
      setTimeout(() => {
        onSuccess?.();
        handleClose();
      }, 2000);
    } catch (err) {
      setError(err.message);
      setStatus('error');
    }
  };

  const handleClose = () => {
    setForm({ to_account_id: '', amount: '' });
    setStatus('idle');
    setError('');
    onClose();
  };

  return (
    <AnimatePresence>
      {isOpen && (
        <>
          {/* Overlay */}
          <motion.div
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            exit={{ opacity: 0 }}
            className="fixed inset-0 bg-black/60 backdrop-blur-sm z-40"
            onClick={handleClose}
          />

          {/* Modal */}
          <motion.div
            initial={{ opacity: 0, scale: 0.9, y: 20 }}
            animate={{ opacity: 1, scale: 1, y: 0 }}
            exit={{ opacity: 0, scale: 0.9, y: 20 }}
            transition={{ type: 'spring', damping: 25, stiffness: 300 }}
            className="fixed inset-0 z-50 flex items-center justify-center p-4"
          >
            <div className="glass-card w-full max-w-md p-6 relative">
              {/* Close */}
              <button
                onClick={handleClose}
                className="absolute top-4 right-4 p-1.5 rounded-lg hover:bg-white/5 text-slate-400 hover:text-white transition-colors cursor-pointer"
              >
                <X className="w-5 h-5" />
              </button>

              <AnimatePresence mode="wait">
                {status === 'success' ? (
                  /* ── Success State ────────────────────────── */
                  <motion.div
                    key="success"
                    initial={{ opacity: 0, x: 60 }}
                    animate={{ opacity: 1, x: 0 }}
                    exit={{ opacity: 0, x: -60 }}
                    transition={{ type: 'spring', damping: 20 }}
                    className="py-8 text-center"
                  >
                    <motion.div
                      initial={{ scale: 0 }}
                      animate={{ scale: 1 }}
                      transition={{ delay: 0.1, type: 'spring', stiffness: 200 }}
                    >
                      <CheckCircle2 className="w-16 h-16 text-emerald-400 mx-auto" />
                    </motion.div>
                    <motion.h3
                      initial={{ opacity: 0, y: 10 }}
                      animate={{ opacity: 1, y: 0 }}
                      transition={{ delay: 0.2 }}
                      className="text-xl font-bold text-white mt-4"
                    >
                      Transfer Successful!
                    </motion.h3>
                    <motion.p
                      initial={{ opacity: 0 }}
                      animate={{ opacity: 1 }}
                      transition={{ delay: 0.3 }}
                      className="text-slate-400 mt-2 text-sm"
                    >
                      ₹{parseFloat(form.amount).toLocaleString('en-IN')} has been sent.
                    </motion.p>
                    <motion.div
                      initial={{ scaleX: 0 }}
                      animate={{ scaleX: 1 }}
                      transition={{ delay: 0.4, duration: 0.8, ease: 'easeOut' }}
                      className="mt-6 h-1 rounded-full bg-linear-to-r from-emerald-500 to-emerald-400 origin-left"
                    />
                  </motion.div>
                ) : (
                  /* ── Form State ──────────────────────────── */
                  <motion.div
                    key="form"
                    initial={{ opacity: 0, x: -20 }}
                    animate={{ opacity: 1, x: 0 }}
                    exit={{ opacity: 0, x: -60 }}
                  >
                    <div className="mb-6">
                      <h3 className="text-xl font-bold text-white flex items-center gap-2">
                        <Send className="w-5 h-5 text-emerald-400" />
                        Quick Transfer
                      </h3>
                      <p className="text-sm text-slate-500 mt-1">
                        Send funds to another account instantly
                      </p>
                    </div>

                    {status === 'error' && error && (
                      <motion.div
                        initial={{ opacity: 0, height: 0 }}
                        animate={{ opacity: 1, height: 'auto' }}
                        className="mb-4 bg-rose-500/10 border border-rose-500/20 rounded-xl px-4 py-3 text-rose-400 text-sm flex items-start gap-2"
                      >
                        <AlertCircle className="w-4 h-4 mt-0.5 shrink-0" />
                        {error}
                      </motion.div>
                    )}

                    <form onSubmit={handleSubmit} className="space-y-4">
                      <div className="space-y-1.5">
                        <label className="text-sm font-medium text-slate-400">To Account ID</label>
                        <input
                          type="text"
                          placeholder="e.g. 550e8400-e29b-41d4-a716..."
                          className="input-glass w-full font-mono text-sm"
                          value={form.to_account_id}
                          onChange={(e) => setForm({ ...form, to_account_id: e.target.value })}
                          required
                        />
                      </div>

                      <div className="space-y-1.5">
                        <label className="text-sm font-medium text-slate-400">Amount (₹)</label>
                        <input
                          type="number"
                          step="0.01"
                          min="0.01"
                          placeholder="0.00"
                          className="input-glass w-full text-2xl font-bold"
                          value={form.amount}
                          onChange={(e) => setForm({ ...form, amount: e.target.value })}
                          required
                        />
                      </div>

                      <motion.button
                        whileHover={{ scale: 1.01 }}
                        whileTap={{ scale: 0.98 }}
                        disabled={status === 'loading'}
                        type="submit"
                        className="w-full py-3 px-4 rounded-xl font-semibold text-white
                                   bg-linear-to-r from-emerald-600 to-emerald-500
                                   hover:from-emerald-500 hover:to-emerald-400
                                   disabled:opacity-50 disabled:cursor-not-allowed
                                   transition-all duration-300 flex items-center justify-center gap-2
                                   shadow-lg shadow-emerald-500/20 cursor-pointer"
                      >
                        {status === 'loading' ? (
                          <Loader2 className="w-5 h-5 animate-spin" />
                        ) : (
                          <>
                            Send Funds
                            <Send className="w-4 h-4" />
                          </>
                        )}
                      </motion.button>
                    </form>
                  </motion.div>
                )}
              </AnimatePresence>
            </div>
          </motion.div>
        </>
      )}
    </AnimatePresence>
  );
}
