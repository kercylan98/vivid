'use client';

import Link from 'next/link';
import { motion } from 'motion/react';
import { buttonVariants } from 'fumadocs-ui/components/ui/button';
import { BookOpen, ExternalLink, Github } from 'lucide-react';

const trustStripItems = ['完整 Actor 模型', '位置透明', '类型安全', '监督容错'];

const container = {
  hidden: { opacity: 0 },
  visible: {
    opacity: 1,
    transition: {
      staggerChildren: 0.08,
      delayChildren: 0.1,
    },
  },
};

const item = {
  hidden: { opacity: 0, y: 24 },
  visible: {
    opacity: 1,
    y: 0,
    transition: { duration: 0.5, ease: [0.22, 1, 0.36, 1] as const },
  },
};

const lineGrowth = {
  hidden: { scaleX: 0, opacity: 0.6 },
  visible: {
    scaleX: 1,
    opacity: 1,
    transition: { duration: 0.6, delay: 0.4, ease: [0.22, 1, 0.36, 1] as const },
  },
};

export function HomeHero() {
  return (
    <section className="relative flex min-h-screen flex-col items-center justify-center overflow-hidden px-4 pt-12 pb-24 text-center md:pt-16 md:pb-32 lg:pt-20 lg:pb-40">
      <motion.div
        className="vivid-hero-glow mx-auto w-full max-w-4xl"
        variants={container}
        initial="hidden"
        animate="visible"
      >
        <motion.div variants={item} className="relative inline-block">
          <h1 className="text-5xl font-bold tracking-tight md:text-7xl lg:text-8xl">
            <span className="vivid-hero-title-gradient">Vivid</span>
            <motion.span
              className="absolute bottom-0 left-1/2 mt-2 h-1 w-20 -translate-x-1/2 rounded-full bg-[var(--vivid-gold)] md:mt-3 md:h-1 md:w-28"
              variants={lineGrowth}
              style={{ transformOrigin: 'center' }}
            />
          </h1>
        </motion.div>

        <motion.p
          variants={item}
          className="vivid-hero-tagline mx-auto mt-8 font-semibold tracking-tight text-fd-foreground md:mt-12"
        >
          构建可扩展、高并发的
          <br className="hidden sm:block" />
          <span className="text-[var(--vivid-gold-soft)]">分布式应用</span>，从 Actor 开始
        </motion.p>

        <motion.p
          variants={item}
          className="mx-auto mt-6 max-w-2xl text-base leading-relaxed text-fd-muted-foreground md:mt-8 md:text-lg"
        >
          高性能、类型安全的 <strong className="text-fd-foreground">Go 语言 Actor 模型实现库</strong>
          ，完整 Actor 系统、消息传递、远程通信与监督策略，本地与分布式同一套 API。
        </motion.p>

        <motion.div variants={item} className="mt-12 md:mt-16">
          <div className="vivid-hero-diagram" aria-hidden>
            <div className="node">Actor</div>
            <div className="arrow" />
            <div className="node node-msg">消息</div>
            <div className="arrow" />
            <div className="node">Actor</div>
          </div>
        </motion.div>

        <motion.div variants={item} className="vivid-trust-strip mt-8 md:mt-10">
          {trustStripItems.map((text) => (
            <span key={text}>{text}</span>
          ))}
        </motion.div>

        <motion.div
          variants={item}
          className="mt-12 flex flex-wrap items-center justify-center gap-4 md:mt-14"
        >
          <Link
            href="/docs"
            className={`${buttonVariants({ color: 'primary', size: 'sm' })} vivid-btn-glow gap-2 px-8 py-4 text-base shadow-[0_0_24px_rgba(212,175,55,0.25)] md:px-10 md:py-4`}
          >
            <BookOpen className="size-5" />
            开始阅读文档
          </Link>
          <a
            href="https://pkg.go.dev/github.com/kercylan98/vivid"
            target="_blank"
            rel="noopener noreferrer"
            className={`${buttonVariants({ color: 'secondary', size: 'sm' })} vivid-btn-outline-hover gap-2 px-8 py-4 text-base md:px-10`}
          >
            <ExternalLink className="size-5" />
            API 文档
          </a>
          <a
            href="https://github.com/kercylan98/vivid"
            target="_blank"
            rel="noopener noreferrer"
            className={`${buttonVariants({ color: 'ghost', size: 'sm' })} vivid-btn-outline-hover gap-2 px-8 py-4 text-base md:px-10`}
          >
            <Github className="size-5" />
            GitHub
          </a>
        </motion.div>
      </motion.div>
    </section>
  );
}
