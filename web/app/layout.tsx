import './globals.css';
import type { Metadata } from 'next';

export const metadata: Metadata = {
  title: 'Arcpay | Borderless Super-App & Merchant OS',
  description: 'Stablecoin-native payments, viral splits, and POS settlement on Arc L1',
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en">
      <body className="bg-background text-zinc-100 min-h-screen antialiased selection:bg-emerald-500/20 selection:text-emerald-400">
        {children}
      </body>
    </html>
  );
}
