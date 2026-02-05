'use client';
import SearchDialog from '@/components/search';
import { ThemeTransitionWrapper } from '@/components/theme-transition-wrapper';
import { RootProvider } from 'fumadocs-ui/provider/next';
import { type ReactNode } from 'react';

export function Provider({ children }: { children: ReactNode }) {
  return (
    <RootProvider search={{ SearchDialog }}>
      <ThemeTransitionWrapper>{children}</ThemeTransitionWrapper>
    </RootProvider>
  );
}
