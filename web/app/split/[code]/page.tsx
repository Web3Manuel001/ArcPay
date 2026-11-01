'use client';

import React, { useState, useEffect } from 'react';
import { useParams } from 'next/navigation';
import { 
  Split, 
  CheckCircle2, 
  CreditCard, 
  Wallet, 
  RefreshCw, 
  ArrowLeft 
} from 'lucide-react';

const rawEnv = process.env.NEXT_PUBLIC_API_URL || 'https://arcpay-core.onrender.com/api/v1';
const cleanBase = rawEnv.replace(/\/+$/, '');
const API_BASE = cleanBase.endsWith('/api/v1') ? cleanBase : `${cleanBase}/api/v1`;

export default function SplitCheckoutPage() {
  const params = useParams();
  const splitCode = params.code as string;

  const [session, setSession] = useState<any>(null);
  const [loading, setLoading] = useState(true);
  const [payerName, setPayerName] = useState('');
  const [payLoading, setPayLoading] = useState(false);
  const [paidSuccess, setPaidSuccess] = useState(false);
  const [errorMsg, setErrorMsg] = useState<string | null>(null);

  useEffect(() => {
    if (splitCode) {
      fetchSplit();
    }
  }, [splitCode]);

  const fetchSplit = async () => {
    try {
      const res = await fetch(`${API_BASE}/consumer/split/${splitCode}`);
      const data = await res.json();
      if (data.success) {
        setSession(data.data);
      } else {
        setErrorMsg('Split session not found or expired');
      }
    } catch (e: any) {
      setErrorMsg('Failed to load split session');
    } finally {
      setLoading(false);
    }
  };

  const handlePayViaPaystack = async () => {
    if (!payerName) {
      alert('Please enter your name or phone number');
      return;
    }
    setPayLoading(true);
    try {
      // 1. Mark and settle split on backend
      const res = await fetch(`${API_BASE}/consumer/split/${splitCode}/pay`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          payer_identifier: payerName,
          payment_method: 'PAYSTACK'
        })
      });
      const data = await res.json();
      if (data.success) {
        setPaidSuccess(true);
        fetchSplit();
      } else {
        alert(data.error || 'Payment failed');
      }
    } catch (e: any) {
      alert(e.message);
    } finally {
      setPayLoading(false);
    }
  };

  if (loading) {
    return (
      <div className="min-h-screen bg-background text-white flex items-center justify-center font-mono text-xs">
        <RefreshCw className="w-5 h-5 animate-spin text-emerald-400 mr-2" />
        <span>Loading ArcSplit Checkout...</span>
      </div>
    );
  }

  if (errorMsg || !session) {
    return (
      <div className="min-h-screen bg-background text-white flex flex-col items-center justify-center p-4 font-mono text-center">
        <div className="p-4 bg-red-500/10 border border-red-500/20 text-red-400 rounded-xl max-w-sm">
          {errorMsg || 'Split session expired'}
        </div>
        <a href="/" className="mt-4 text-xs text-zinc-400 hover:text-white flex items-center gap-1">
          <ArrowLeft className="w-4 h-4" /> Go to Arcpay
        </a>
      </div>
    );
  }

  return (
    <div className="max-w-md mx-auto min-h-screen px-4 py-8 flex flex-col justify-between font-sans">
      <div>
        {/* Header */}
        <div className="flex items-center justify-between pb-6 border-b border-surfaceBorder">
          <div className="flex items-center gap-2">
            <div className="w-8 h-8 rounded-lg bg-emerald-500/10 border border-emerald-500/20 flex items-center justify-center text-emerald-400 font-bold">
              ₳
            </div>
            <div>
              <h1 className="text-sm font-semibold tracking-tight text-white">ARCPAY SPLIT</h1>
              <p className="text-[10px] text-zinc-500 font-mono tracking-wider">SETTLED ON ARC L1</p>
            </div>
          </div>
          <span className="text-xs font-mono text-emerald-400 bg-emerald-500/10 px-2.5 py-1 rounded-full border border-emerald-500/20">
            {session.status}
          </span>
        </div>

        {/* Request Card */}
        <div className="mt-6 p-6 rounded-2xl bg-surface border border-surfaceBorder text-center space-y-3">
          <div className="w-12 h-12 rounded-full bg-emerald-500/10 border border-emerald-500/20 mx-auto flex items-center justify-center text-emerald-400">
            <Split className="w-6 h-6" />
          </div>
          <div>
            <div className="text-xs font-mono text-zinc-400">REQUEST FROM</div>
            <div className="text-base font-bold text-white font-mono">{session.creator_tag}</div>
          </div>
          <p className="text-xs text-zinc-300 italic max-w-xs mx-auto">
            "{session.narration}"
          </p>
          <div className="pt-2 border-t border-surfaceBorder/60">
            <div className="text-xs font-mono text-zinc-500">YOUR SHARE</div>
            <div className="text-3xl font-mono font-bold text-white mt-1">
              ${session.share_per_person_usd.toFixed(2)}
            </div>
            <div className="text-xs font-mono text-emerald-400 mt-0.5">
              ≈ ₦{session.share_per_person_ngn.toLocaleString()} NGN
            </div>
          </div>
        </div>

        {/* Payment Action */}
        <div className="mt-6 p-5 rounded-2xl bg-surface border border-surfaceBorder space-y-4">
          {paidSuccess ? (
            <div className="text-center py-6 space-y-2">
              <CheckCircle2 className="w-12 h-12 text-emerald-400 mx-auto" />
              <div className="text-base font-bold text-white font-mono">YOUR SHARE IS SETTLED!</div>
              <p className="text-xs text-zinc-400">
                ${session.share_per_person_usd.toFixed(2)} was credited directly to {session.creator_tag}.
              </p>
            </div>
          ) : (
            <>
              <div>
                <label className="text-[11px] text-zinc-500 font-mono">YOUR NAME OR PHONE</label>
                <input
                  type="text"
                  value={payerName}
                  onChange={(e) => setPayerName(e.target.value)}
                  placeholder="e.g. John Doe or +234..."
                  className="mt-1 w-full bg-zinc-950 border border-surfaceBorder rounded-xl p-3 text-sm font-mono text-white focus:border-emerald-500/50"
                />
              </div>

              <button
                onClick={handlePayViaPaystack}
                disabled={payLoading}
                className="w-full bg-emerald-500 text-zinc-950 font-semibold py-3.5 rounded-xl text-sm flex items-center justify-center gap-2 hover:bg-emerald-400 transition-colors disabled:opacity-50"
              >
                {payLoading ? <RefreshCw className="w-4 h-4 animate-spin" /> : <CreditCard className="w-4 h-4" />}
                <span>Pay ₦{session.share_per_person_ngn.toLocaleString()} (Card / Bank)</span>
              </button>

              <div className="text-center text-[11px] font-mono text-zinc-500">
                🔒 Instant settlement via Paystack rails
              </div>
            </>
          )}
        </div>
      </div>

      {/* Footer */}
      <div className="mt-8 pt-4 border-t border-surfaceBorder flex items-center justify-between text-[11px] font-mono text-zinc-600">
        <span>Arcpay Protocol</span>
        <a href="/" className="text-emerald-500/80 hover:underline">Open Arcpay App</a>
      </div>
    </div>
  );
}
