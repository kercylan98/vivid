'use client';

import Link from 'next/link';
import { Banner } from 'fumadocs-ui/components/banner';
import { Card } from 'fumadocs-ui/components/card';
import { buttonVariants } from 'fumadocs-ui/components/ui/button';
import {
  Zap,
  MessageSquare,
  Globe,
  Shield,
  Clock,
  Type,
  BookOpen,
  Github,
  Terminal,
  ArrowRight,
  Layers,
  FileText,
} from 'lucide-react';
import { HomeHero } from '@/components/home-hero';
import { HomeSection, HomeSectionStagger, HomeSectionItem } from '@/components/home-section';
import { motion } from 'motion/react';

const featureCards: Array<{
  icon: typeof Zap;
  title: string;
  description: string;
  href: string;
}> = [
  {
    icon: Zap,
    title: '完整 Actor 模型',
    description: 'ActorSystem、ActorContext、ActorRef 等核心抽象，树状层级与生命周期，类型安全的消息传递',
    href: '/docs/introduction/what-is-vivid',
  },
  {
    icon: MessageSquare,
    title: '灵活消息传递',
    description: 'Tell（发后即忘）与 Ask（请求-响应）、Reply、Future；PipeTo、Entrust 等进阶能力',
    href: '/docs/basics/message-delivery',
  },
  {
    icon: Globe,
    title: '网络透明与 Remoting',
    description: '本地与远程同一套 API，ActorRef 按地址自动路由，内置跨节点通信与编解码',
    href: '/docs/config/remoting',
  },
  {
    icon: Shield,
    title: '监督与容错',
    description: 'OneForOne / OneForAll，重启、停止、恢复与升级，退避与抖动可配置',
    href: '/docs/config/supervision',
  },
  {
    icon: Clock,
    title: '调度与行为栈',
    description: 'Once / Loop / Cron 调度，Become / UnBecome 行为切换，适合状态机与定时任务',
    href: '/docs/basics/scheduler',
  },
  {
    icon: Type,
    title: '类型安全与错误体系',
    description: '消息与 Future 泛型、预定义错误码、errors.Is/As 与跨节点错误序列化',
    href: '/docs/config/errors',
  },
];

const docLinks: Array<{ title: string; href: string; brief: string }> = [
  { title: '什么是 Vivid', href: '/docs/introduction/what-is-vivid', brief: '定位、核心特性与架构概览' },
  { title: '快速入门', href: '/docs', brief: '安装到第一个 Actor 的完整步骤' },
  { title: 'Actor 引用（ActorRef）', href: '/docs/basics/actor-ref', brief: '寻址与能力句柄、位置透明' },
  { title: '消息投递', href: '/docs/basics/message-delivery', brief: 'Tell、Ask、Reply、Future' },
];

const useCases = [
  '分布式系统与跨节点通信',
  '高并发与异步任务',
  '微服务与 RPC 风格请求-响应',
  '状态机与事件驱动',
  '定时 / 周期任务',
  '容错与优雅停机',
];

export default function HomePage() {
  return (
    <>
      <Banner
        id="dev-status"
        variant="normal"
        height="auto"
        changeLayout={false}
        className="border-b border-fd-border/80 bg-fd-card/80 py-2.5 text-center text-sm backdrop-blur-sm"
      >
        当前处于活跃开发阶段，API 可能发生变更，生产使用前请查看{' '}
        <a
          href="https://github.com/kercylan98/vivid/releases"
          target="_blank"
          rel="noopener noreferrer"
          className="font-medium text-[var(--vivid-gold)] underline underline-offset-2 hover:text-[var(--vivid-gold-soft)]"
        >
          更新日志
        </a>
      </Banner>

      <div className="vivid-home-content">
        <HomeHero />

        {/* 设计理念：淡底区分，留白充足 */}
        <section className="vivid-section-alt py-20 md:py-24">
          <div className="mx-auto max-w-4xl px-6 text-center md:px-8">
            <HomeSection>
              <p className="text-fd-muted-foreground md:text-lg leading-relaxed">
                Actor 通过 <strong className="text-fd-foreground">消息</strong> 与{' '}
                <strong className="text-fd-foreground">引用（ActorRef）</strong> 与外界交互；
                引用将「身份」与「位置」解耦，同一套 API 即可用于本地与远程，实现{' '}
                <strong className="text-fd-foreground">位置透明</strong>。
              </p>
            </HomeSection>
          </div>
        </section>

        {/* 立即尝试：终端风格 */}
        <section className="py-20 md:py-28">
          <div className="mx-auto w-full max-w-6xl px-6 md:px-8">
            <HomeSection>
              <h2 className="vivid-section-title vivid-section-heading vivid-section-heading-accent text-center">
                立即尝试
              </h2>
            </HomeSection>
            <HomeSection delay={0.08}>
              <p className="mx-auto mt-6 max-w-xl text-center text-fd-muted-foreground md:mt-7 md:text-lg">
                一条命令安装，按文档创建系统与 Actor，即可完成消息收发
              </p>
            </HomeSection>
            <HomeSection delay={0.12}>
              <div className="mx-auto mt-12 max-w-2xl md:mt-16">
                <div className="vivid-terminal-window">
                <div className="title-bar">
                  <span className="dot" />
                  <span className="dot" />
                  <span className="dot" />
                  <span className="ml-2">Terminal</span>
                </div>
                <div className="border-t-0 border border-fd-border bg-[var(--vivid-terminal-bg)] p-5 md:p-6">
                  <div className="flex items-start gap-3 font-mono text-sm">
                    <span className="select-none text-[var(--vivid-gold-muted)]">$</span>
                    <code className="block flex-1 text-[var(--vivid-gold-soft)]">
                      go get github.com/kercylan98/vivid
                    </code>
                  </div>
                  <div className="mt-3 flex items-start gap-3 font-mono text-xs text-fd-muted-foreground md:text-sm">
                    <span className="select-none text-[var(--vivid-gold-muted)]">&gt;</span>
                    <code className="block flex-1 text-[var(--vivid-gold-muted)]">
                      system.ActorOf(&amp;EchoActor&#123;&#125;, vivid.WithActorName(&quot;echo&quot;))
                    </code>
                  </div>
                  <p className="mt-4 text-sm leading-relaxed text-fd-muted-foreground">
                    在代码中通过 <code className="rounded bg-fd-muted/80 px-1.5 py-0.5 text-[var(--vivid-gold-soft)]">bootstrap.NewActorSystem()</code> 创建系统，用 <code className="rounded bg-fd-muted/80 px-1.5 py-0.5 text-[var(--vivid-gold-soft)]">ActorOf</code> 创建 Actor，Tell/Ask 收发消息。完整步骤与可运行示例见{' '}
                    <Link
                      href="/docs"
                      className="font-medium text-[var(--vivid-gold-soft)] underline underline-offset-2 hover:text-[var(--vivid-gold)]"
                    >
                      快速入门
                    </Link>
                    。
                  </p>
                </div>
                </div>
              </div>
              <p className="mt-4 flex items-center justify-center gap-2 text-sm text-fd-muted-foreground">
                <ArrowRight className="size-4" />
                概念与进阶能力请从文档左侧导航进入
              </p>
            </HomeSection>
          </div>
        </section>

        {/* 核心特性：交替背景 */}
        <section className="vivid-section-alt py-20 md:py-28">
          <div className="mx-auto w-full max-w-6xl px-6 md:px-8">
            <HomeSection>
              <h2 className="vivid-section-title vivid-section-heading vivid-section-heading-accent text-center">
                核心特性
              </h2>
            </HomeSection>
            <HomeSection delay={0.06}>
              <p className="mx-auto mt-6 max-w-2xl text-center text-fd-muted-foreground md:mt-8 md:text-lg">
                Actor 模型完整实现，从本地到分布式、从消息投递到监督容错一站到位
              </p>
            </HomeSection>
            <HomeSectionStagger className="mt-12 grid gap-6 sm:grid-cols-2 lg:mt-16 lg:grid-cols-3 lg:gap-8" staggerDelay={0.08}>
              {featureCards.map((item) => (
                <HomeSectionItem key={item.title}>
                  <motion.div className="h-full" whileHover={{ y: -6 }} transition={{ duration: 0.25, ease: [0.22, 1, 0.36, 1] }}>
                    <Card
                    icon={<item.icon className="size-5 text-[var(--vivid-gold)]" />}
                    title={item.title}
                    description={item.description}
                    href={item.href}
                    className="h-full border-fd-border/80 hover:border-[var(--vivid-gold-muted)]"
                  />
                  </motion.div>
                </HomeSectionItem>
              ))}
            </HomeSectionStagger>
          </div>
        </section>

        {/* 推荐阅读 */}
        <section className="py-20 md:py-28">
          <div className="mx-auto w-full max-w-6xl px-6 md:px-8">
            <HomeSection>
              <h2 className="vivid-section-title vivid-section-heading vivid-section-heading-accent text-center">
                推荐阅读
              </h2>
            </HomeSection>
            <HomeSection delay={0.06}>
              <p className="mx-auto mt-6 max-w-xl text-center text-fd-muted-foreground md:mt-7 md:text-lg">
                从概念到实践，按文档顺序快速建立完整认知
              </p>
            </HomeSection>
            <HomeSectionStagger className="mt-12 grid gap-5 sm:grid-cols-2 lg:mt-14 lg:gap-6" staggerDelay={0.07}>
              {docLinks.map((doc) => (
                <HomeSectionItem key={doc.href}>
                  <Link
                    href={doc.href}
                    className="vivid-doc-card vivid-card-hover flex items-start gap-4 rounded-2xl border border-fd-border/80 bg-fd-card p-6 transition-colors hover:border-[var(--vivid-gold-muted)] hover:bg-fd-muted/30 md:p-7"
                  >
                  <span className="flex h-12 w-12 shrink-0 items-center justify-center rounded-xl bg-[var(--vivid-gold)]/15 text-[var(--vivid-gold)]">
                    <FileText className="size-6" />
                  </span>
                  <div className="min-w-0 flex-1">
                    <h3 className="text-lg font-semibold text-fd-foreground">{doc.title}</h3>
                    <p className="mt-1.5 text-sm text-fd-muted-foreground">{doc.brief}</p>
                  </div>
                  <ArrowRight className="size-5 shrink-0 text-fd-muted-foreground" />
                </Link>
                </HomeSectionItem>
              ))}
            </HomeSectionStagger>
          </div>
        </section>

        {/* 适用场景 */}
        <section className="vivid-section-alt py-20 md:py-28">
          <div className="mx-auto w-full max-w-6xl px-6 md:px-8">
            <HomeSection>
              <h2 className="vivid-section-title vivid-section-heading vivid-section-heading-accent text-center">
                适用场景
              </h2>
            </HomeSection>
            <HomeSection delay={0.06}>
              <p className="mx-auto mt-6 max-w-xl text-center text-fd-muted-foreground md:mt-7 md:text-lg">
                基于消息与引用的并发与分布式能力，覆盖多种架构需求
              </p>
            </HomeSection>
            <HomeSectionStagger className="mt-12 flex flex-wrap justify-center gap-4 lg:mt-14" staggerDelay={0.04}>
              {useCases.map((label) => (
                <HomeSectionItem key={label}>
                  <span className="inline-flex items-center rounded-full border border-fd-border/80 bg-fd-card px-5 py-2.5 text-sm font-medium text-fd-foreground shadow-sm">
                    <Layers className="mr-2 size-4 text-[var(--vivid-gold-muted)]" />
                    {label}
                  </span>
                </HomeSectionItem>
              ))}
            </HomeSectionStagger>
          </div>
        </section>
      </div>

      {/* CTA：独立于内容区，全宽、强视觉冲击 + 进入动效 */}
      <motion.section
        className="vivid-cta-section"
        initial="hidden"
        whileInView="visible"
        viewport={{ once: true, margin: '-80px' }}
        variants={{
          hidden: { opacity: 0 },
          visible: {
            opacity: 1,
            transition: { staggerChildren: 0.12, delayChildren: 0.08 },
          },
        }}
      >
        <div className="vivid-cta-section-inner">
          <motion.h2
            className="vivid-cta-title vivid-hero-title-gradient vivid-section-title vivid-section-heading-accent"
            variants={{ hidden: { opacity: 0, y: 24 }, visible: { opacity: 1, y: 0 } }}
            transition={{ duration: 0.5, ease: [0.22, 1, 0.36, 1] }}
          >
            准备好构建分布式应用了吗？
          </motion.h2>
          <motion.p
            className="mt-5 text-lg text-fd-muted-foreground md:mt-6 md:text-xl"
            variants={{ hidden: { opacity: 0, y: 20 }, visible: { opacity: 1, y: 0 } }}
            transition={{ duration: 0.45, ease: [0.22, 1, 0.36, 1] }}
          >
            从文档与快速入门开始，或到 GitHub 查看源码与参与贡献
          </motion.p>
          <motion.div
            className="mt-10 flex flex-wrap justify-center gap-4 md:mt-12 md:gap-5"
            variants={{ hidden: { opacity: 0, y: 16 }, visible: { opacity: 1, y: 0 } }}
            transition={{ duration: 0.5, ease: [0.22, 1, 0.36, 1] }}
          >
            <Link
              href="/docs"
              className={`${buttonVariants({ color: 'primary', size: 'sm' })} vivid-btn-glow gap-2 px-8 py-4 text-base shadow-[0_0_24px_rgba(212,175,55,0.3)] md:px-10`}
            >
              <BookOpen className="size-5" />
              阅读文档
            </Link>
            <a
              href="https://github.com/kercylan98/vivid"
              target="_blank"
              rel="noopener noreferrer"
              className={`${buttonVariants({ color: 'outline', size: 'sm' })} vivid-btn-outline-hover gap-2 border-[var(--vivid-gold-muted)] px-8 py-4 text-base text-fd-foreground hover:border-[var(--vivid-gold)] hover:bg-[var(--vivid-gold-subtle)] md:px-10`}
            >
              <Github className="size-5" />
              查看 GitHub
            </a>
          </motion.div>
        </div>
      </motion.section>
    </>
  );
}
