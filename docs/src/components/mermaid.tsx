'use client';

import { useTheme } from 'next-themes';
import { useCallback, useEffect, useId, useRef, useState } from 'react';

type MermaidProps = {
  /** 图表代码（与 Fumadocs / remarkMdxMermaid 兼容） */
  chart?: string;
  /** 图表代码（兼容现有 MDX 使用的 diagram） */
  diagram?: string;
  title?: string;
};

// eslint-disable-next-line @typescript-eslint/no-explicit-any
const cache = new Map<string, Promise<any>>();

function loadMermaid() {
  const key = 'mermaid';
  let p = cache.get(key);
  if (!p) {
    p = import('mermaid');
    cache.set(key, p);
  }
  return p;
}

export function Mermaid({ chart, diagram, title }: MermaidProps) {
  const content = (diagram ?? chart ?? '').replaceAll('\\n', '\n').trim();
  const id = useId().replace(/:/g, '');
  const containerRef = useRef<HTMLDivElement>(null);
  const [result, setResult] = useState<{ svg: string; bind?: (el: HTMLElement) => void } | null>(null);
  const [error, setError] = useState<string | null>(null);
  const { resolvedTheme } = useTheme();

  const bindRef = useCallback(
    (el: HTMLDivElement | null) => {
      (containerRef as React.MutableRefObject<HTMLDivElement | null>).current = el;
      if (el && result?.bind) result.bind(el);
    },
    [result?.bind],
  );

  useEffect(() => {
    if (!content) return;
    setError(null);
    setResult(null);
    let cancelled = false;

    loadMermaid()
      .then((mermaidModule) => {
        if (cancelled) return;
        const mermaid = mermaidModule.default;
        mermaid.initialize({
          startOnLoad: false,
          securityLevel: 'loose',
          fontFamily: 'inherit',
          themeCSS: 'margin: 0;',
          theme: resolvedTheme === 'dark' ? 'dark' : 'default',
        });
        const uniqueId = `mermaid-${id}-${resolvedTheme ?? 'light'}`;
        return mermaid.render(uniqueId, content);
      })
      .then((rendered) => {
        if (cancelled || !rendered) return;
        const { svg, bindFunctions } = rendered;
        setResult({ svg, bind: bindFunctions ?? undefined });
      })
      .catch((err) => {
        if (!cancelled) setError(String(err?.message ?? err));
      });

    return () => {
      cancelled = true;
    };
  }, [content, id, resolvedTheme]);

  useEffect(() => {
    if (result?.bind && containerRef.current) result.bind(containerRef.current);
  }, [result?.bind, result?.svg]);

  if (error) {
    return (
      <div className="my-6 rounded-lg border border-red-200 bg-red-50 p-4 dark:border-red-800 dark:bg-red-950/30">
        <p className="text-sm font-medium text-red-800 dark:text-red-200">Mermaid 渲染失败</p>
        <pre className="mt-2 overflow-auto text-xs text-red-700 dark:text-red-300">{error}</pre>
      </div>
    );
  }

  if (!result?.svg) {
    return (
      <figure className="mermaid-figure my-6">
        {title ? (
          <figcaption className="mb-2 text-center text-sm text-zinc-500 dark:text-zinc-400">{title}</figcaption>
        ) : null}
        <div
          className="mermaid-diagram flex min-h-[120px] items-center justify-center rounded-xl border border-[var(--color-fd-border)] bg-[var(--color-fd-muted)]/50 px-4 py-5"
          aria-busy="true"
        >
          <span className="text-sm text-zinc-400 dark:text-zinc-500">加载图表…</span>
        </div>
      </figure>
    );
  }

  return (
    <figure className="mermaid-figure my-6">
      {title ? (
        <figcaption className="mb-2 text-center text-sm text-zinc-500 dark:text-zinc-400">{title}</figcaption>
      ) : null}
      <div
        ref={bindRef}
        className="mermaid-diagram flex justify-center rounded-xl border border-[var(--color-fd-border)] bg-[var(--color-fd-muted)]/30 px-4 py-5 [&>svg]:max-w-full [&>svg]:overflow-visible"
        dangerouslySetInnerHTML={{ __html: result.svg }}
      />
    </figure>
  );
}
