import { Inter } from 'next/font/google';
import { Provider } from '@/components/provider';
import './global.css';
import type { Metadata } from 'next';

const inter = Inter({
  subsets: ['latin'],
});

const baseUrl =
  process.env.METADATA_BASE ||
  (process.env.BASE_PATH ? 'https://kercylan98.github.io' : 'http://localhost:3000');

export const metadata: Metadata = {
  metadataBase: new URL(baseUrl),
};

export default function Layout({ children }: LayoutProps<'/'>) {
  return (
    <html lang="en" className={inter.className} suppressHydrationWarning>
      <body className="flex flex-col min-h-screen">
        <Provider>{children}</Provider>
      </body>
    </html>
  );
}
