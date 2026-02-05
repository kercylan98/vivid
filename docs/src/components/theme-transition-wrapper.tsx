'use client';

import { useTheme } from 'next-themes';
import { useCallback, useEffect, useRef, useState, type ReactNode } from 'react';
import html2canvas from 'html2canvas-pro';

/** 扩散/回退速度：vmax 每毫秒，覆盖满屏(320vmax) 的时长 = 320 / 此值 */
const RADIATE_SPEED_VMAX_PER_MS = 0.32;
const RADIATE_FULL_SIZE_VMAX = 320;
const EXPAND_DURATION_MS = Math.round(RADIATE_FULL_SIZE_VMAX / RADIATE_SPEED_VMAX_PER_MS);

const SCROLL_MATCH_TOLERANCE = 2;

const SCROLL_LOCK_CLASS = 'vivid-theme-transition-no-scroll';

export function ThemeTransitionWrapper({ children }: { children: ReactNode }) {
  const { resolvedTheme, setTheme } = useTheme();
  const prevTheme = useRef<string | undefined>(undefined);
  const [overlayState, setOverlayState] = useState<
    null | 'expand' | 'revert'
  >(null);
  const [radiateTheme, setRadiateTheme] = useState<string | null>(null);
  const [radiateCenter, setRadiateCenter] = useState({ x: 0, y: 0 });
  const [screenshotUrl, setScreenshotUrl] = useState<string | null>(null);
  const clickPosRef = useRef({ x: 0, y: 0 });
  const timersRef = useRef<ReturnType<typeof setTimeout>[]>([]);
  const overlayStateRef = useRef(overlayState);
  const radiateThemeRef = useRef(radiateTheme);
  const resolvedThemeRef = useRef(resolvedTheme);
  type CacheEntry = { url: string; scrollX: number; scrollY: number };
  const screenshotCacheRef = useRef<{ light?: CacheEntry; dark?: CacheEntry }>({});
  const capturingRef = useRef<string | null>(null);
  const overlayRef = useRef<HTMLDivElement>(null);
  const [revertFromSize, setRevertFromSize] = useState<string | null>(null);
  const [revertDurationMs, setRevertDurationMs] = useState(EXPAND_DURATION_MS);
  overlayStateRef.current = overlayState;
  radiateThemeRef.current = radiateTheme;
  resolvedThemeRef.current = resolvedTheme;

  const preventScroll = useCallback((e: WheelEvent | TouchEvent) => e.preventDefault(), []);
  const scrollLockRef = useRef<(() => void) | null>(null);
  const lockScroll = useCallback(() => {
    if (scrollLockRef.current) return;
    const opts = { passive: false } as AddEventListenerOptions;
    document.addEventListener('wheel', preventScroll as (e: Event) => void, opts);
    document.addEventListener('touchmove', preventScroll as (e: Event) => void, opts);
    document.documentElement.classList.add(SCROLL_LOCK_CLASS);
    document.body.classList.add(SCROLL_LOCK_CLASS);
    scrollLockRef.current = () => {
      document.removeEventListener('wheel', preventScroll as (e: Event) => void, opts);
      document.removeEventListener('touchmove', preventScroll as (e: Event) => void, opts);
      document.documentElement.classList.remove(SCROLL_LOCK_CLASS);
      document.body.classList.remove(SCROLL_LOCK_CLASS);
      scrollLockRef.current = null;
    };
  }, [preventScroll]);
  const unlockScroll = useCallback(() => {
    scrollLockRef.current?.();
  }, []);

  const clearTimers = useCallback(() => {
    timersRef.current.forEach((t) => clearTimeout(t));
    timersRef.current = [];
  }, []);

  const clearOverlay = useCallback(() => {
    overlayStateRef.current = null;
    setOverlayState(null);
    setRadiateTheme(null);
    setScreenshotUrl(null);
    setRevertFromSize(null);
    setRevertDurationMs(EXPAND_DURATION_MS);
    document.documentElement.classList.remove('vivid-theme-icon-transition');
    unlockScroll();
  }, [unlockScroll]);

  const doCapture = useCallback(() =>
    html2canvas(document.body, {
      x: window.scrollX,
      y: window.scrollY,
      width: window.innerWidth,
      height: window.innerHeight,
    }), []);

  const applyTransitionWithUrl = useCallback(
    (url: string | null) => {
      const fromTheme = resolvedThemeRef.current ?? 'light';
      const targetTheme = fromTheme === 'dark' ? 'light' : 'dark';
      overlayStateRef.current = 'expand';
      radiateThemeRef.current = fromTheme;
      document.documentElement.classList.add('vivid-theme-icon-transition');
      document.querySelector('[data-theme-toggle]')?.setAttribute('data-icon-transitioning', 'true');
      setScreenshotUrl(url);
      setRadiateCenter(clickPosRef.current);
      setRadiateTheme(fromTheme);
      setOverlayState('expand');
      lockScroll();
      prevTheme.current = targetTheme;
      setTheme(targetTheme);
      document.querySelector('[data-theme-toggle]')?.removeAttribute('data-icon-transitioning');
    },
    [lockScroll, setTheme]
  );

  const captureMousedown = useCallback((e: MouseEvent) => {
    const target = e.target as HTMLElement;
    if (target.closest?.('[data-theme-toggle]')) {
      clickPosRef.current = { x: e.clientX, y: e.clientY };
    }
  }, []);

  const precacheTheme = useCallback(
    (theme: string) => {
      const key = theme as 'light' | 'dark';
      if (capturingRef.current === theme) return;
      capturingRef.current = theme;
      const run = () => {
        doCapture()
          .then((canvas) => {
            const url = canvas.toDataURL('image/png');
            screenshotCacheRef.current[key] = {
              url,
              scrollX: window.scrollX,
              scrollY: window.scrollY,
            };
            capturingRef.current = null;
          })
          .catch(() => {
            capturingRef.current = null;
          });
      };
      if (typeof requestIdleCallback !== 'undefined') {
        requestIdleCallback(run, { timeout: 200 });
      } else {
        setTimeout(run, 0);
      }
    },
    [doCapture]
  );

  const captureMouseenter = useCallback(() => {
    const theme = resolvedThemeRef.current ?? 'light';
    const cache = screenshotCacheRef.current;
    const key = theme as 'light' | 'dark';
    if (cache[key]) return;
    precacheTheme(theme);
  }, [precacheTheme]);

  const handleToggleClick = useCallback(
    (e: MouseEvent) => {
      const target = e.target as HTMLElement;
      if (!target.closest?.('[data-theme-toggle]')) return;
      e.preventDefault();
      e.stopPropagation();

      if (overlayStateRef.current === 'revert') return;

      if (overlayStateRef.current === 'expand') {
        overlayStateRef.current = 'revert';
        clearTimers();
        const el = overlayRef.current;
        let startSize = '320vmax';
        let durationMs = EXPAND_DURATION_MS;
        if (el) {
          const current = getComputedStyle(el).getPropertyValue('--vivid-radiate-size').trim();
          if (current) {
            startSize = current;
            const currentPx = parseFloat(current);
            if (Number.isFinite(currentPx)) {
              const fullPx = (RADIATE_FULL_SIZE_VMAX * Math.max(window.innerWidth, window.innerHeight)) / 100;
              if (fullPx > 0) {
                const currentVmax = RADIATE_FULL_SIZE_VMAX * (currentPx / fullPx);
                durationMs = Math.max(100, Math.round(currentVmax / RADIATE_SPEED_VMAX_PER_MS));
              }
            }
          }
        }
        setRevertFromSize(startSize);
        setRevertDurationMs(durationMs);
        setOverlayState('revert');
        const revertedTheme = radiateThemeRef.current ?? 'light';
        const t = setTimeout(() => {
          setTheme(revertedTheme);
          document.querySelector('[data-theme-toggle]')?.removeAttribute('data-icon-transitioning');
          prevTheme.current = revertedTheme;
          clearOverlay();
          overlayStateRef.current = null;
          setTimeout(() => precacheTheme(revertedTheme), 80);
        }, durationMs);
        timersRef.current = [t];
        return;
      }

      const fromTheme = resolvedTheme ?? 'light';
      const targetTheme = fromTheme === 'dark' ? 'light' : 'dark';
      const key = fromTheme as 'light' | 'dark';
      const cached = screenshotCacheRef.current[key];
      const scrollOk =
        cached &&
        Math.abs(window.scrollX - cached.scrollX) <= SCROLL_MATCH_TOLERANCE &&
        Math.abs(window.scrollY - cached.scrollY) <= SCROLL_MATCH_TOLERANCE;

      if (scrollOk && cached) {
        applyTransitionWithUrl(cached.url);
        return;
      }

      doCapture()
        .then((canvas) => {
          const url = canvas.toDataURL('image/png');
          screenshotCacheRef.current[key] = {
            url,
            scrollX: window.scrollX,
            scrollY: window.scrollY,
          };
          applyTransitionWithUrl(url);
        })
        .catch(() => {
          applyTransitionWithUrl(null);
        });
    },
    [resolvedTheme, setTheme, clearTimers, clearOverlay, doCapture, applyTransitionWithUrl, precacheTheme]
  );

  const onMouseOver = useCallback(
    (e: MouseEvent) => {
      const el = (e.target as HTMLElement).closest?.('[data-theme-toggle]');
      if (!el) return;
      const related = e.relatedTarget as Node | null;
      if (related && el.contains(related)) return;
      captureMouseenter();
    },
    [captureMouseenter]
  );

  useEffect(() => {
    document.addEventListener('mousedown', captureMousedown, true);
    document.addEventListener('click', handleToggleClick, true);
    document.addEventListener('mouseover', onMouseOver, true);
    return () => {
      document.removeEventListener('mousedown', captureMousedown, true);
      document.removeEventListener('click', handleToggleClick, true);
      document.removeEventListener('mouseover', onMouseOver, true);
    };
  }, [captureMousedown, handleToggleClick, onMouseOver]);

  useEffect(() => {
    if (resolvedTheme == null) return;
    if (overlayState === 'revert') {
      prevTheme.current = resolvedTheme;
      return;
    }
    if (overlayState === 'expand') return;
    prevTheme.current = resolvedTheme;
  }, [resolvedTheme, overlayState]);

  // 扩散/回退动画结束时移除遮罩（与动画时长一致，无多余延迟）
  const handleAnimationEnd = useCallback(
    (e: React.AnimationEvent<HTMLDivElement>) => {
      if (e.animationName?.includes('circle-size-expand') || e.animationName?.includes('circle-size-revert')) {
        clearTimers();
        prevTheme.current = resolvedTheme ?? prevTheme.current;
        clearOverlay();
        const theme = resolvedTheme ?? 'light';
        if (e.animationName?.includes('circle-size-expand')) {
          setTimeout(() => precacheTheme(theme), 0);
        }
      }
    },
    [clearTimers, clearOverlay, resolvedTheme, precacheTheme]
  );

  const showOverlay = overlayState !== null && radiateTheme !== null;
  const durationMs =
    overlayState === 'revert' ? revertDurationMs : EXPAND_DURATION_MS;

  return (
    <>
      {children}
      {showOverlay && (
        <div
          ref={overlayRef}
          className={`vivid-theme-radiate-overlay vivid-theme-radiate-${overlayState}`}
          data-theme={radiateTheme}
          style={
            {
              '--vivid-radiate-x': `${radiateCenter.x}px`,
              '--vivid-radiate-y': `${radiateCenter.y}px`,
              '--vivid-radiate-duration': `${durationMs}ms`,
              ...(overlayState === 'revert' && revertFromSize
                ? { '--vivid-radiate-size-start': revertFromSize }
                : {}),
              ...(screenshotUrl
                ? {
                    backgroundImage: `url(${screenshotUrl})`,
                    backgroundSize: 'cover',
                    backgroundPosition: '0 0',
                  }
                : {}),
            } as React.CSSProperties
          }
          onAnimationEnd={handleAnimationEnd}
          aria-hidden
        />
      )}
    </>
  );
}
