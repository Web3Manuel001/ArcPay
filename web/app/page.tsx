'use client';

import React, { useState, useEffect, useRef } from 'react';
import { 
  ArrowUpRight, 
  ArrowDownLeft,
  Split, 
  Smartphone, 
  Store, 
  CheckCircle2, 
  Copy, 
  Send, 
  Sparkles, 
  RefreshCw,
  ShieldCheck,
  LogOut,
  Lock,
  Plus,
  CreditCard,
  Share2,
  X
} from 'lucide-react';

const rawEnv = process.env.NEXT_PUBLIC_API_URL || 'https://arcpay-core.onrender.com/api/v1';
const cleanBase = rawEnv.replace(/\/+$/, '');
const API_BASE = cleanBase.endsWith('/api/v1') ? cleanBase : `${cleanBase}/api/v1`;

export default function ArcpayApp() {
  const [activeTab, setActiveTab] = useState<'wallet' | 'split' | 'airtime' | 'pos'>('wallet');

  // Auth & Session State
  const [currentUser, setCurrentUser] = useState<any>(null);
  const [showAuthModal, setShowAuthModal] = useState(false);
  const [isLoginMode, setIsLoginMode] = useState(false);

  // Auth Form State
  const [authName, setAuthName] = useState('');
  const [authTag, setAuthTag] = useState('');
  const [authPhone, setAuthPhone] = useState('+234');
  const [authPin, setAuthPin] = useState('');
  const [authLoading, setAuthLoading] = useState(false);

  // Financial State
  const [balanceUSD, setBalanceUSD] = useState<number>(0.00);
  const [history, setHistory] = useState<any[]>([]);

  // Modals & Receipts
  const [selectedReceipt, setSelectedReceipt] = useState<any>(null);
  const [toastMessage, setToastMessage] = useState<{ text: string; type: 'success' | 'error' } | null>(null);

  // Deposit State
  const [depositAmountNGN, setDepositAmountNGN] = useState('16500');
  const [depositLoading, setDepositLoading] = useState(false);
  const [showDepositModal, setShowDepositModal] = useState(false);

  // P2P State
  const [recipientTag, setRecipientTag] = useState('');
  const [transferAmount, setTransferAmount] = useState('');
  const [transferLoading, setTransferLoading] = useState(false);

  // ArcSplit State
  const [jevPrompt, setJevPrompt] = useState('Split $60 with @chidi and @kofi for dinner at RSVP');
  const [splitLoading, setSplitLoading] = useState(false);
  const [activeSplit, setActiveSplit] = useState<any>(null);

  // Airtime State
  const [airtimePhone, setAirtimePhone] = useState('+2348099998888');
  const [airtimeOperator, setAirtimeOperator] = useState('MTN');
  const [airtimeAmount, setAirtimeAmount] = useState('2.00');
  const [airtimeLoading, setAirtimeLoading] = useState(false);

  // POS State
  const [posAmount, setPosAmount] = useState('15.00');
  const [posTable, setPosTable] = useState('Table 4');
  const [activeInvoice, setActiveInvoice] = useState<any>(null);
  const [posPaid, setPosPaid] = useState(false);
  const wsRef = useRef<WebSocket | null>(null);

  // Auto-toast dismiss
  const showToast = (text: string, type: 'success' | 'error' = 'success') => {
    setToastMessage({ text, type });
    setTimeout(() => setToastMessage(null), 4000);
  };

  // Load User & Auto-Refresh Balance
  useEffect(() => {
    const saved = localStorage.getItem('arcpay_user');
    if (saved) {
      try {
        const u = JSON.parse(saved);
        setCurrentUser(u);
        fetchBalanceAndHistory(u.account_id);

        // Check Paystack Callback
        const params = new URLSearchParams(window.location.search);
        const ref = params.get('verify_ref') || params.get('reference');
        if (ref) {
          verifyPaystackDeposit(ref, u.account_id);
        }
      } catch (e) {}
    } else {
      setShowAuthModal(true);
    }
  }, []);

  // 5-second Polling Loop for Live Balance & History
  useEffect(() => {
    if (!currentUser) return;
    const interval = setInterval(() => {
      fetchBalanceAndHistory(currentUser.account_id);
    }, 5000);
    return () => clearInterval(interval);
  }, [currentUser]);

  const fetchBalanceAndHistory = async (accountId: string) => {
    try {
      // Balance
      const bRes = await fetch(`${API_BASE}/accounts/${accountId}/balance`);
      const bData = await bRes.json();
      if (bData.success) {
        setBalanceUSD(bData.data.balance_amount);
      }
      // History
      const hRes = await fetch(`${API_BASE}/accounts/${accountId}/history`);
      const hData = await hRes.json();
      if (hData.success) {
        setHistory(hData.data || []);
      }
    } catch (e) {}
  };

  const verifyPaystackDeposit = async (ref: string, accountId: string) => {
    try {
      const res = await fetch(`${API_BASE}/consumer/deposit/verify/${ref}`);
      const data = await res.json();
      if (data.success) {
        showToast(`🎉 Deposit Confirmed! Credited $${data.data.credited_usdc.toFixed(2)} USDC`);
        fetchBalanceAndHistory(accountId);
        window.history.replaceState({}, document.title, window.location.pathname);
      }
    } catch (e) {}
  };

  const playPOSChime = () => {
    try {
      const ctx = new (window.AudioContext || (window as any).webkitAudioContext)();
      const osc = ctx.createOscillator();
      const gain = ctx.createGain();
      osc.type = 'sine';
      osc.frequency.setValueAtTime(587.33, ctx.currentTime);
      osc.frequency.exponentialRampToValueAtTime(880, ctx.currentTime + 0.15);
      gain.gain.setValueAtTime(0.2, ctx.currentTime);
      gain.gain.exponentialRampToValueAtTime(0.01, ctx.currentTime + 0.4);
      osc.connect(gain);
      gain.connect(ctx.destination);
      osc.start();
      osc.stop(ctx.currentTime + 0.4);
    } catch (e) {}
  };

  const apiCall = async (endpoint: string, options: RequestInit) => {
    const res = await fetch(`${API_BASE}${endpoint}`, options);
    const text = await res.text();
    try {
      return JSON.parse(text);
    } catch (e) {
      throw new Error(`Server returned HTTP ${res.status}: ${text.slice(0, 80)}`);
    }
  };

  // Paystack Deposit
  const handlePaystackDeposit = async () => {
    if (!currentUser) return;
    setDepositLoading(true);
    try {
      const data = await apiCall('/consumer/deposit/initialize', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          tag: currentUser.arc_tag,
          amount_ngn: parseFloat(depositAmountNGN)
        })
      });
      if (data.success && data.data.authorization_url) {
        window.location.href = data.data.authorization_url;
      } else {
        showToast(data.error || 'Failed to initialize deposit', 'error');
      }
    } catch (err: any) {
      showToast(err.message, 'error');
    } finally {
      setDepositLoading(false);
    }
  };

  // Auth Handler
  const handleAuthSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setAuthLoading(true);
    try {
      if (isLoginMode) {
        const data = await apiCall('/auth/login', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ tag_or_phone: authTag, pin: authPin })
        });
        if (data.success) {
          setCurrentUser(data.data.user);
          localStorage.setItem('arcpay_user', JSON.stringify(data.data.user));
          setShowAuthModal(false);
          fetchBalanceAndHistory(data.data.user.account_id);
          showToast(`Welcome back, ${data.data.user.arc_tag}`);
        } else {
          showToast(data.error || 'Login failed', 'error');
        }
      } else {
        const data = await apiCall('/auth/register', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({
            full_name: authName,
            phone_number: authPhone,
            arc_tag: authTag,
            pin: authPin,
            corridor: 'NG'
          })
        });
        if (data.success) {
          setCurrentUser(data.data.user);
          localStorage.setItem('arcpay_user', JSON.stringify(data.data.user));
          setShowAuthModal(false);
          setBalanceUSD(0.00);
          showToast(`Account created! Welcome ${data.data.user.arc_tag}`);
        } else {
          showToast(data.error || 'Registration failed', 'error');
        }
      }
    } catch (err: any) {
      showToast(err.message, 'error');
    } finally {
      setAuthLoading(false);
    }
  };

  const handleLogout = () => {
    localStorage.removeItem('arcpay_user');
    setCurrentUser(null);
    setShowAuthModal(true);
  };

  // P2P Transfer
  const handleTransfer = async () => {
    if (!currentUser || !recipientTag || !transferAmount) return;
    setTransferLoading(true);
    try {
      const data = await apiCall('/consumer/transfers/tag', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          idempotency_key: `p2p_${Date.now()}`,
          sender_tag: currentUser.arc_tag,
          recipient_tag: recipientTag,
          amount_units: Math.round(parseFloat(transferAmount) * 1_000_000),
          narration: 'P2P Transfer'
        })
      });
      if (data.success) {
        showToast(`Sent $${transferAmount} to ${recipientTag}`);
        setSelectedReceipt({
          title: 'P2P Transfer',
          amountUSD: parseFloat(transferAmount),
          narration: `Sent to ${recipientTag}`,
          reference: data.data.journal_id,
          date: new Date().toLocaleString(),
          isDebit: true
        });
        setTransferAmount('');
        setRecipientTag('');
        fetchBalanceAndHistory(currentUser.account_id);
      } else {
        showToast(data.error || 'Transfer failed', 'error');
      }
    } catch (err: any) {
      showToast(err.message, 'error');
    } finally {
      setTransferLoading(false);
    }
  };

  // ArcSplit
  const handleCreateSplit = async () => {
    if (!currentUser || !jevPrompt) return;
    setSplitLoading(true);
    try {
      const data = await apiCall('/consumer/split/create', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          creator_tag: currentUser.arc_tag,
          prompt: jevPrompt
        })
      });
      if (data.success) {
        setActiveSplit(data.data);
        showToast('ArcSplit link created!');
      } else {
        showToast(data.error || 'Failed to parse split', 'error');
      }
    } catch (err: any) {
      showToast(err.message, 'error');
    } finally {
      setSplitLoading(false);
    }
  };

  // Airtime
  const handleBuyAirtime = async () => {
    if (!currentUser) return;
    setAirtimeLoading(true);
    try {
      const data = await apiCall('/consumer/vas/airtime', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          idempotency_key: `airtime_${Date.now()}`,
          sender_tag: currentUser.arc_tag,
          phone_number: airtimePhone,
          operator: airtimeOperator,
          amount_usdc: parseFloat(airtimeAmount),
          corridor: 'NG'
        })
      });
      if (data.success) {
        showToast(`Airtime sent to ${airtimePhone}!`);
        setSelectedReceipt({
          title: 'Airtime Recharge',
          amountUSD: parseFloat(airtimeAmount),
          narration: `${airtimeOperator} Airtime to ${airtimePhone}`,
          reference: data.data.journal_entry_id,
          date: new Date().toLocaleString(),
          isDebit: true
        });
        fetchBalanceAndHistory(currentUser.account_id);
      } else {
        showToast(data.error || 'Airtime purchase failed', 'error');
      }
    } catch (err: any) {
      showToast(err.message, 'error');
    } finally {
      setAirtimeLoading(false);
    }
  };

  // POS Invoice
  const handleCreateInvoice = async () => {
    if (!currentUser) return;
    setPosPaid(false);
    try {
      const data = await apiCall('/business/pos/invoices', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          merchant_account_id: currentUser.account_id,
          amount_units: Math.round(parseFloat(posAmount) * 1_000_000),
          table_number: posTable
        })
      });
      if (data.success) {
        const inv = data.data;
        setActiveInvoice(inv);

        if (wsRef.current) wsRef.current.close();
        const wsHost = API_BASE.replace('http://', 'ws://').replace('https://', 'wss://');
        const ws = new WebSocket(`${wsHost}/business/pos/ws/${inv.id}`);
        wsRef.current = ws;

        ws.onmessage = (event) => {
          const payload = JSON.parse(event.data);
          if (payload.status === 'PAID') {
            setPosPaid(true);
            playPOSChime();
            showToast('POS Payment Settled!');
            fetchBalanceAndHistory(currentUser.account_id);
          }
        };
      }
    } catch (err: any) {
      showToast(err.message, 'error');
    }
  };

  return (
    <div className="max-w-md mx-auto min-h-screen px-4 py-8 flex flex-col justify-between font-sans">
      {/* IN-APP TOAST */}
      {toastMessage && (
        <div className={`fixed top-4 left-1/2 -translate-x-1/2 z-50 px-4 py-2.5 rounded-xl text-xs font-mono shadow-lg border transition-all ${
          toastMessage.type === 'success' 
            ? 'bg-zinc-900 border-emerald-500/50 text-emerald-400' 
            : 'bg-zinc-900 border-red-500/50 text-red-400'
        }`}>
          {toastMessage.text}
        </div>
      )}

      {/* RECEIPT MODAL */}
      {selectedReceipt && (
        <div className="fixed inset-0 bg-black/80 backdrop-blur-sm z-50 flex items-center justify-center p-4">
          <div className="bg-surface border border-surfaceBorder rounded-2xl w-full max-w-sm p-6 space-y-4">
            <div className="flex items-center justify-between pb-2 border-b border-surfaceBorder">
              <span className="text-xs font-mono text-zinc-500">TRANSACTION RECEIPT</span>
              <button onClick={() => setSelectedReceipt(null)} className="text-zinc-500 hover:text-white">
                <X className="w-4 h-4" />
              </button>
            </div>

            <div className="text-center py-2 space-y-1">
              <div className="w-10 h-10 rounded-full bg-emerald-500/10 border border-emerald-500/20 text-emerald-400 mx-auto flex items-center justify-center font-bold">
                ✓
              </div>
              <div className="text-2xl font-mono font-bold text-white">
                {selectedReceipt.isDebit ? '-' : '+'}${selectedReceipt.amountUSD.toFixed(2)} USDC
              </div>
              <div className="text-xs font-mono text-zinc-500">
                ≈ ₦{(selectedReceipt.amountUSD * 1650).toLocaleString()} NGN
              </div>
            </div>

            <div className="p-3 bg-zinc-950 rounded-xl border border-surfaceBorder font-mono text-xs space-y-2">
              <div className="flex justify-between text-zinc-400">
                <span>Description:</span>
                <span className="text-white text-right max-w-[180px] truncate">{selectedReceipt.narration}</span>
              </div>
              <div className="flex justify-between text-zinc-400">
                <span>Date & Time:</span>
                <span className="text-white">{selectedReceipt.date}</span>
              </div>
              <div className="flex justify-between text-zinc-400">
                <span>Network:</span>
                <span className="text-emerald-400">Circle Arc L1</span>
              </div>
              <div className="flex justify-between text-zinc-400">
                <span>Status:</span>
                <span className="text-white">SETTLED (FINAL)</span>
              </div>
            </div>

            <a
              href={`https://wa.me/?text=${encodeURIComponent(`Receipt from Arcpay: ${selectedReceipt.narration} of $${selectedReceipt.amountUSD.toFixed(2)} USDC (Settled on Arc L1). Ref: ${selectedReceipt.reference}`)}`}
              target="_blank"
              rel="noreferrer"
              className="w-full bg-emerald-500 text-zinc-950 font-semibold py-3 rounded-xl text-xs flex items-center justify-center gap-2 hover:bg-emerald-400 transition-colors"
            >
              <Share2 className="w-4 h-4" />
              <span>Share to WhatsApp</span>
            </a>
          </div>
        </div>
      )}

      {/* AUTH MODAL */}
      {showAuthModal && (
        <div className="fixed inset-0 bg-black/80 backdrop-blur-sm z-50 flex items-center justify-center p-4">
          <div className="bg-surface border border-surfaceBorder rounded-2xl w-full max-w-sm p-6 space-y-4">
            <div className="text-center space-y-1">
              <div className="w-10 h-10 rounded-xl bg-emerald-500/10 border border-emerald-500/20 text-emerald-400 font-bold mx-auto flex items-center justify-center text-lg">
                ₳
              </div>
              <h2 className="text-base font-semibold text-white">
                {isLoginMode ? 'Welcome back' : 'Create Arcpay Account'}
              </h2>
              <p className="text-xs text-zinc-400">
                {isLoginMode ? 'Enter your ArcTag or phone and PIN' : 'Instant onboarding · Zero seed phrases'}
              </p>
            </div>

            <form onSubmit={handleAuthSubmit} className="space-y-3 font-mono text-xs">
              {!isLoginMode && (
                <div>
                  <label className="text-[10px] text-zinc-500">FULL NAME</label>
                  <input
                    type="text"
                    required
                    value={authName}
                    onChange={(e) => setAuthName(e.target.value)}
                    placeholder="Emmanuel Okon"
                    className="mt-1 w-full bg-zinc-950 border border-surfaceBorder rounded-xl p-3 text-white focus:border-emerald-500/50"
                  />
                </div>
              )}

              <div>
                <label className="text-[10px] text-zinc-500">ARCTAG</label>
                <input
                  type="text"
                  required
                  value={authTag}
                  onChange={(e) => setAuthTag(e.target.value)}
                  placeholder="$yourname"
                  className="mt-1 w-full bg-zinc-950 border border-surfaceBorder rounded-xl p-3 text-white focus:border-emerald-500/50"
                />
              </div>

              {!isLoginMode && (
                <div>
                  <label className="text-[10px] text-zinc-500">PHONE NUMBER</label>
                  <input
                    type="text"
                    required
                    value={authPhone}
                    onChange={(e) => setAuthPhone(e.target.value)}
                    placeholder="+2348012345678"
                    className="mt-1 w-full bg-zinc-950 border border-surfaceBorder rounded-xl p-3 text-white focus:border-emerald-500/50"
                  />
                </div>
              )}

              <div>
                <label className="text-[10px] text-zinc-500">4-DIGIT PIN</label>
                <input
                  type="password"
                  maxLength={6}
                  required
                  value={authPin}
                  onChange={(e) => setAuthPin(e.target.value)}
                  placeholder="••••"
                  className="mt-1 w-full bg-zinc-950 border border-surfaceBorder rounded-xl p-3 text-white tracking-widest text-center focus:border-emerald-500/50"
                />
              </div>

              <button
                type="submit"
                disabled={authLoading}
                className="w-full bg-white text-zinc-950 font-semibold py-3.5 rounded-xl text-xs flex items-center justify-center gap-2 hover:bg-zinc-200 transition-colors disabled:opacity-50 mt-2"
              >
                {authLoading ? <RefreshCw className="w-4 h-4 animate-spin" /> : <Lock className="w-4 h-4" />}
                <span>{isLoginMode ? 'Sign In' : 'Create Account (10s)'}</span>
              </button>
            </form>

            <div className="text-center pt-2 border-t border-surfaceBorder">
              <button
                onClick={() => setIsLoginMode(!isLoginMode)}
                className="text-xs text-zinc-400 hover:text-white"
              >
                {isLoginMode ? "Don't have an account? Sign up" : 'Already have an account? Sign in'}
              </button>
            </div>
          </div>
        </div>
      )}

      {/* DEPOSIT MODAL */}
      {showDepositModal && (
        <div className="fixed inset-0 bg-black/80 backdrop-blur-sm z-50 flex items-center justify-center p-4">
          <div className="bg-surface border border-surfaceBorder rounded-2xl w-full max-w-sm p-6 space-y-4">
            <div className="flex items-center justify-between pb-2 border-b border-surfaceBorder">
              <div className="flex items-center gap-2">
                <CreditCard className="w-4 h-4 text-emerald-400" />
                <h3 className="text-sm font-semibold text-white">Deposit via Paystack</h3>
              </div>
              <button onClick={() => setShowDepositModal(false)} className="text-zinc-500 hover:text-white text-xs font-mono">
                ✕
              </button>
            </div>

            <div className="space-y-3 font-mono text-xs">
              <div>
                <label className="text-[10px] text-zinc-500">AMOUNT IN NAIRA (NGN)</label>
                <input
                  type="number"
                  value={depositAmountNGN}
                  onChange={(e) => setDepositAmountNGN(e.target.value)}
                  className="mt-1 w-full bg-zinc-950 border border-surfaceBorder rounded-xl p-3 text-white focus:border-emerald-500/50"
                />
              </div>

              <div className="grid grid-cols-3 gap-2">
                {['5000', '16500', '50000'].map((amt) => (
                  <button
                    key={amt}
                    onClick={() => setDepositAmountNGN(amt)}
                    className={`py-1.5 rounded-lg border text-[11px] transition-colors ${
                      depositAmountNGN === amt 
                        ? 'bg-emerald-500/10 border-emerald-500/40 text-emerald-400' 
                        : 'bg-zinc-950 border-surfaceBorder text-zinc-400 hover:text-white'
                    }`}
                  >
                    ₦{parseInt(amt).toLocaleString()}
                  </button>
                ))}
              </div>

              <div className="p-3 bg-zinc-950 rounded-xl border border-surfaceBorder text-[11px] text-zinc-400 space-y-1">
                <div className="flex justify-between">
                  <span>Rate:</span>
                  <span className="text-white">$1.00 = ₦1,650</span>
                </div>
                <div className="flex justify-between">
                  <span>You Receive:</span>
                  <span className="text-emerald-400 font-bold">
                    ${(parseFloat(depositAmountNGN || '0') / 1650).toFixed(2)} USDC
                  </span>
                </div>
              </div>

              <button
                onClick={handlePaystackDeposit}
                disabled={depositLoading}
                className="w-full bg-emerald-500 text-zinc-950 font-semibold py-3.5 rounded-xl text-xs flex items-center justify-center gap-2 hover:bg-emerald-400 transition-colors disabled:opacity-50"
              >
                {depositLoading ? <RefreshCw className="w-4 h-4 animate-spin" /> : <CreditCard className="w-4 h-4" />}
                <span>Proceed to Checkout</span>
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Main Container */}
      <div>
        {/* Header */}
        <div className="flex items-center justify-between pb-6 border-b border-surfaceBorder">
          <div className="flex items-center gap-2">
            <div className="w-8 h-8 rounded-lg bg-emerald-500/10 border border-emerald-500/20 flex items-center justify-center text-emerald-400 font-bold">
              ₳
            </div>
            <div>
              <h1 className="text-sm font-semibold tracking-tight text-white">ARCPAY</h1>
              <p className="text-[10px] text-zinc-500 font-mono tracking-wider">SETTLED ON ARC L1</p>
            </div>
          </div>

          {currentUser ? (
            <div className="flex items-center gap-2">
              <div className="flex items-center gap-1.5 bg-surface px-3 py-1.5 rounded-full border border-surfaceBorder">
                <span className="w-2 h-2 rounded-full bg-emerald-400 animate-pulse"></span>
                <span className="text-xs font-mono font-medium text-zinc-300">{currentUser.arc_tag}</span>
              </div>
              <button onClick={handleLogout} className="p-1.5 text-zinc-500 hover:text-white" title="Log out">
                <LogOut className="w-4 h-4" />
              </button>
            </div>
          ) : (
            <button
              onClick={() => setShowAuthModal(true)}
              className="text-xs font-mono bg-zinc-800 hover:bg-zinc-700 px-3 py-1.5 rounded-full border border-zinc-700 text-white"
            >
              Sign In
            </button>
          )}
        </div>

        {/* Balance Card */}
        <div className="mt-6 p-6 rounded-2xl bg-surface border border-surfaceBorder relative overflow-hidden">
          <div className="flex items-center justify-between text-xs font-mono text-zinc-400 uppercase tracking-wider">
            <span>Digital Dollar Balance</span>
            {currentUser && (
              <span className="flex items-center gap-1 text-[10px] text-emerald-400 bg-emerald-500/10 px-2 py-0.5 rounded border border-emerald-500/20">
                <ShieldCheck className="w-3 h-3" /> TIER 0 ($100/DAY)
              </span>
            )}
          </div>

          <div className="mt-2 flex items-baseline justify-between">
            <div className="flex items-baseline gap-2">
              <span className="text-3xl font-mono font-bold tracking-tight text-white">${balanceUSD.toFixed(2)}</span>
              <span className="text-xs font-mono text-emerald-400 font-medium">USDC</span>
            </div>

            {currentUser && (
              <button
                onClick={() => setShowDepositModal(true)}
                className="flex items-center gap-1.5 px-3.5 py-2 bg-emerald-500 hover:bg-emerald-400 text-zinc-950 font-semibold font-mono text-xs rounded-xl transition-all shadow-sm"
              >
                <Plus className="w-4 h-4 stroke-[3]" />
                <span>Add Money</span>
              </button>
            )}
          </div>

          <div className="mt-1 text-xs font-mono text-zinc-500">
            ≈ ₦{(balanceUSD * 1650).toLocaleString()} NGN
          </div>
        </div>

        {/* Navigation Tabs */}
        <div className="mt-6 grid grid-cols-4 gap-1 p-1 bg-surface rounded-xl border border-surfaceBorder">
          {[
            { id: 'wallet', label: 'Send', icon: ArrowUpRight },
            { id: 'split', label: 'Split', icon: Split },
            { id: 'airtime', label: 'Airtime', icon: Smartphone },
            { id: 'pos', label: 'POS', icon: Store },
          ].map((tab) => {
            const Icon = tab.icon;
            const active = activeTab === tab.id;
            return (
              <button
                key={tab.id}
                onClick={() => setActiveTab(tab.id as any)}
                className={`flex flex-col items-center gap-1 py-2 rounded-lg text-xs font-medium transition-all ${
                  active 
                    ? 'bg-zinc-800 text-white border border-zinc-700/50 shadow-sm' 
                    : 'text-zinc-500 hover:text-zinc-300'
                }`}
              >
                <Icon className="w-4 h-4" />
                <span>{tab.label}</span>
              </button>
            );
          })}
        </div>

        {/* TAB 1: P2P */}
        {activeTab === 'wallet' && (
          <div className="mt-6 p-5 rounded-2xl bg-surface border border-surfaceBorder space-y-4">
            <h2 className="text-xs font-mono text-zinc-400 uppercase tracking-wider">Social P2P Transfer</h2>
            
            <div>
              <label className="text-[11px] text-zinc-500 font-mono">RECIPIENT ARCTAG</label>
              <input
                type="text"
                value={recipientTag}
                onChange={(e) => setRecipientTag(e.target.value)}
                placeholder="$chidi or $lagosbistro"
                className="mt-1 w-full bg-zinc-950 border border-surfaceBorder rounded-xl px-4 py-3 text-sm font-mono text-white placeholder-zinc-700 focus:border-emerald-500/50"
              />
            </div>

            <div>
              <label className="text-[11px] text-zinc-500 font-mono">AMOUNT (USDC)</label>
              <input
                type="number"
                value={transferAmount}
                onChange={(e) => setTransferAmount(e.target.value)}
                placeholder="0.00"
                className="mt-1 w-full bg-zinc-950 border border-surfaceBorder rounded-xl px-4 py-3 text-sm font-mono text-white placeholder-zinc-700 focus:border-emerald-500/50"
              />
            </div>

            <button
              onClick={handleTransfer}
              disabled={transferLoading}
              className="w-full bg-white text-zinc-950 font-semibold py-3.5 rounded-xl text-sm flex items-center justify-center gap-2 hover:bg-zinc-200 transition-colors disabled:opacity-50"
            >
              {transferLoading ? <RefreshCw className="w-4 h-4 animate-spin" /> : <Send className="w-4 h-4" />}
              <span>Send Instantly</span>
            </button>
          </div>
        )}

        {/* TAB 2: ARCSPLIT */}
        {activeTab === 'split' && (
          <div className="mt-6 p-5 rounded-2xl bg-surface border border-surfaceBorder space-y-4">
            <div className="flex items-center justify-between">
              <h2 className="text-xs font-mono text-zinc-400 uppercase tracking-wider">ArcSplit + Jev AI</h2>
              <span className="text-[10px] font-mono text-emerald-400 bg-emerald-500/10 px-2 py-0.5 rounded border border-emerald-500/20">
                SYSTEM-1 &lt;100MS
              </span>
            </div>

            <div>
              <label className="text-[11px] text-zinc-500 font-mono flex items-center gap-1">
                <Sparkles className="w-3 h-3 text-emerald-400" /> NATURAL LANGUAGE INTENT
              </label>
              <textarea
                value={jevPrompt}
                onChange={(e) => setJevPrompt(e.target.value)}
                rows={3}
                className="mt-1 w-full bg-zinc-950 border border-surfaceBorder rounded-xl p-3 text-sm text-white placeholder-zinc-700 focus:border-emerald-500/50"
              />
            </div>

            <button
              onClick={handleCreateSplit}
              disabled={splitLoading}
              className="w-full bg-emerald-500 text-zinc-950 font-semibold py-3.5 rounded-xl text-sm flex items-center justify-center gap-2 hover:bg-emerald-400 transition-colors disabled:opacity-50"
            >
              {splitLoading ? <RefreshCw className="w-4 h-4 animate-spin" /> : <Split className="w-4 h-4" />}
              <span>Parse & Generate Split Link</span>
            </button>

            {activeSplit && (
              <div className="p-4 rounded-xl bg-zinc-950 border border-surfaceBorder space-y-3 font-mono text-xs">
                <div className="flex justify-between text-zinc-400">
                  <span>Session:</span>
                  <span className="text-white">{activeSplit.split_code}</span>
                </div>
                <div className="flex justify-between text-zinc-400">
                  <span>Share Per Person:</span>
                  <span className="text-emerald-400 font-bold">${activeSplit.share_per_person_usd.toFixed(2)}</span>
                </div>
                <div>
                  <div className="text-[10px] text-zinc-500 mb-1">WHATSAPP SHAREABLE PAY-LINK</div>
                  <div className="flex items-center justify-between bg-zinc-900 p-2.5 rounded-lg border border-zinc-800 text-zinc-300">
                    <span className="truncate">{activeSplit.shareable_link}</span>
                    <button 
                      onClick={() => {
                        navigator.clipboard.writeText(activeSplit.shareable_link);
                        showToast('Link copied to clipboard!');
                      }}
                      className="ml-2 text-zinc-400 hover:text-white"
                    >
                      <Copy className="w-3.5 h-3.5" />
                    </button>
                  </div>
                </div>
              </div>
            )}
          </div>
        )}

        {/* TAB 3: AIRTIME */}
        {activeTab === 'airtime' && (
          <div className="mt-6 p-5 rounded-2xl bg-surface border border-surfaceBorder space-y-4">
            <h2 className="text-xs font-mono text-zinc-400 uppercase tracking-wider">First-Party Airtime Top-Up</h2>

            <div>
              <label className="text-[11px] text-zinc-500 font-mono">OPERATOR</label>
              <select
                value={airtimeOperator}
                onChange={(e) => setAirtimeOperator(e.target.value)}
                className="mt-1 w-full bg-zinc-950 border border-surfaceBorder rounded-xl px-4 py-3 text-sm font-mono text-white focus:border-emerald-500/50"
              >
                <option value="MTN">MTN Nigeria</option>
                <option value="AIRTEL">Airtel</option>
                <option value="SAFARICOM">Safaricom Kenya</option>
                <option value="GLO">Glo</option>
              </select>
            </div>

            <div>
              <label className="text-[11px] text-zinc-500 font-mono">DESTINATION PHONE</label>
              <input
                type="text"
                value={airtimePhone}
                onChange={(e) => setAirtimePhone(e.target.value)}
                className="mt-1 w-full bg-zinc-950 border border-surfaceBorder rounded-xl px-4 py-3 text-sm font-mono text-white focus:border-emerald-500/50"
              />
            </div>

            <div>
              <label className="text-[11px] text-zinc-500 font-mono">AMOUNT (USDC)</label>
              <input
                type="number"
                value={airtimeAmount}
                onChange={(e) => setAirtimeAmount(e.target.value)}
                className="mt-1 w-full bg-zinc-950 border border-surfaceBorder rounded-xl px-4 py-3 text-sm font-mono text-white focus:border-emerald-500/50"
              />
              <span className="text-[10px] text-zinc-500 font-mono mt-1 block">
                ≈ ₦{(parseFloat(airtimeAmount || '0') * 1650).toLocaleString()} local airtime
              </span>
            </div>

            <button
              onClick={handleBuyAirtime}
              disabled={airtimeLoading}
              className="w-full bg-white text-zinc-950 font-semibold py-3.5 rounded-xl text-sm flex items-center justify-center gap-2 hover:bg-zinc-200 transition-colors disabled:opacity-50"
            >
              {airtimeLoading ? <RefreshCw className="w-4 h-4 animate-spin" /> : <Smartphone className="w-4 h-4" />}
              <span>Dispatch Airtime Switch</span>
            </button>
          </div>
        )}

        {/* TAB 4: POS */}
        {activeTab === 'pos' && (
          <div className="mt-6 p-5 rounded-2xl bg-surface border border-surfaceBorder space-y-4">
            <div className="flex items-center justify-between">
              <h2 className="text-xs font-mono text-zinc-400 uppercase tracking-wider">Arcpay Business POS</h2>
              <span className="text-[10px] font-mono text-emerald-400 bg-emerald-500/10 px-2 py-0.5 rounded border border-emerald-500/20">
                0.5% TAKE-RATE
              </span>
            </div>

            <div className="grid grid-cols-2 gap-3">
              <div>
                <label className="text-[11px] text-zinc-500 font-mono">BILL AMOUNT ($)</label>
                <input
                  type="number"
                  value={posAmount}
                  onChange={(e) => setPosAmount(e.target.value)}
                  className="mt-1 w-full bg-zinc-950 border border-surfaceBorder rounded-xl p-3 text-sm font-mono text-white"
                />
              </div>
              <div>
                <label className="text-[11px] text-zinc-500 font-mono">TABLE / REF</label>
                <input
                  type="text"
                  value={posTable}
                  onChange={(e) => setPosTable(e.target.value)}
                  className="mt-1 w-full bg-zinc-950 border border-surfaceBorder rounded-xl p-3 text-sm font-mono text-white"
                />
              </div>
            </div>

            <button
              onClick={handleCreateInvoice}
              className="w-full bg-emerald-500 text-zinc-950 font-semibold py-3 rounded-xl text-sm flex items-center justify-center gap-2 hover:bg-emerald-400 transition-colors"
            >
              <Store className="w-4 h-4" />
              <span>Generate Dynamic QR</span>
            </button>

            {activeInvoice && (
              <div className={`mt-4 p-6 rounded-2xl border transition-all text-center ${
                posPaid 
                  ? 'bg-emerald-950/40 border-emerald-500 text-emerald-400' 
                  : 'bg-zinc-950 border-surfaceBorder text-zinc-300'
              }`}>
                {posPaid ? (
                  <div className="space-y-2 py-4">
                    <CheckCircle2 className="w-12 h-12 text-emerald-400 mx-auto animate-bounce" />
                    <div className="text-lg font-bold font-mono tracking-tight text-white">BILL PAID & SETTLED!</div>
                    <div className="text-xs font-mono text-emerald-300">${posAmount} USDC Received (Arc L1 Finality)</div>
                  </div>
                ) : (
                  <div className="space-y-3">
                    <div className="w-36 h-36 mx-auto bg-white rounded-xl p-2 flex items-center justify-center">
                      <div className="w-full h-full border-4 border-dashed border-zinc-950 flex flex-col items-center justify-center text-zinc-950 font-mono text-[9px] text-center font-bold">
                        <span>SCAN TO PAY</span>
                        <span className="text-[8px] text-zinc-600 mt-1">{activeInvoice.id}</span>
                      </div>
                    </div>
                    <div className="text-xs font-mono font-medium text-white">
                      Scan to pay ${posAmount} USDC ({posTable})
                    </div>
                    <div className="text-[10px] font-mono text-zinc-500 flex items-center justify-center gap-1.5">
                      <span className="w-2 h-2 rounded-full bg-amber-400 animate-pulse"></span>
                      Terminal connected to live WebSocket...
                    </div>
                  </div>
                )}
              </div>
            )}
          </div>
        )}

        {/* TRANSACTION HISTORY FEED */}
        {currentUser && (
          <div className="mt-8 space-y-3">
            <div className="flex items-center justify-between text-xs font-mono text-zinc-400">
              <span className="uppercase tracking-wider">Activity Feed</span>
              <span className="text-[10px] text-zinc-600">LIVE AUTO-SYNC</span>
            </div>

            {history.length === 0 ? (
              <div className="p-6 text-center text-xs font-mono text-zinc-600 border border-surfaceBorder rounded-2xl bg-surface">
                No transactions yet. Add money or send to get started.
              </div>
            ) : (
              <div className="space-y-2">
                {history.map((tx) => {
                  const isCredit = tx.direction === 'CREDIT';
                  return (
                    <div
                      key={tx.id}
                      onClick={() => setSelectedReceipt({
                        title: tx.reference_type,
                        amountUSD: tx.amount_usd,
                        narration: tx.narration,
                        reference: tx.id,
                        date: new Date(tx.created_at).toLocaleString(),
                        isDebit: !isCredit
                      })}
                      className="p-3.5 bg-surface hover:bg-zinc-900 border border-surfaceBorder rounded-xl flex items-center justify-between cursor-pointer transition-colors"
                    >
                      <div className="flex items-center gap-3">
                        <div className={`w-8 h-8 rounded-lg flex items-center justify-center ${
                          isCredit ? 'bg-emerald-500/10 text-emerald-400' : 'bg-zinc-800 text-zinc-300'
                        }`}>
                          {isCredit ? <ArrowDownLeft className="w-4 h-4" /> : <ArrowUpRight className="w-4 h-4" />}
                        </div>
                        <div>
                          <div className="text-xs font-medium text-white max-w-[160px] truncate">{tx.narration}</div>
                          <div className="text-[10px] font-mono text-zinc-500">{new Date(tx.created_at).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })}</div>
                        </div>
                      </div>

                      <div className="text-right">
                        <div className={`text-xs font-mono font-bold ${
                          isCredit ? 'text-emerald-400' : 'text-white'
                        }`}>
                          {isCredit ? '+' : '-'}${tx.amount_usd.toFixed(2)}
                        </div>
                        <div className="text-[10px] font-mono text-zinc-500">
                          {isCredit ? 'CREDIT' : 'DEBIT'}
                        </div>
                      </div>
                    </div>
                  );
                })}
              </div>
            )}
          </div>
        )}
      </div>

      {/* Footer */}
      <div className="mt-8 pt-4 border-t border-surfaceBorder flex items-center justify-between text-[11px] font-mono text-zinc-600">
        <span>Arcpay Protocol v1.0</span>
        <span className="text-emerald-500/80">Circle Arc L1</span>
      </div>
    </div>
  );
}
