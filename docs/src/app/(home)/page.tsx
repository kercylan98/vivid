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
  ExternalLink,
  Github,
  Terminal,
  ArrowRight,
} from 'lucide-react';

const featureCards = [
  { icon: Zap, title: '完整 Actor 模型', description: 'Actor 系统、上下文、引用等核心抽象，类型安全的消息传递' },
  { icon: MessageSquare, title: '灵活消息传递', description: 'Tell / Ask 与 PipeTo，支持请求-响应与发后即忘' },
  { icon: Globe, title: '网络透明与 Remoting', description: 'ActorRef 自动路由本地/远程，内置跨节点通信' },
  { icon: Shield, title: '监督策略', description: 'OneForOne / OneForAll，重启、停止、恢复与升级' },
  { icon: Clock, title: '调度与行为栈', description: '定时/周期/Cron 调度，行为动态切换与状态机' },
  { icon: Type, title: '类型安全', description: '充分利用 Go 类型系统，错误链与标准 error 处理' },
] as const;

export default function HomePage() {
  return (
    <>
      {/* Banner 固定在顶端导航条下方 */}
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
        {/* Hero：更大气 */}
        <section className="flex flex-col items-center justify-center py-20 text-center md:py-28 lg:py-32">
          <div className="mx-auto w-full max-w-2xl">
            <h1 className="vivid-opacity-start vivid-animate-in-up text-4xl font-bold tracking-tight md:text-6xl lg:text-7xl">
              <span className="relative inline-block">
                Vivid
                <span
                  className="absolute bottom-0 left-1/2 mt-2 h-0.5 w-16 -translate-x-1/2 rounded-full bg-[var(--vivid-gold)] md:mt-2.5 md:w-24"
                  style={{ animation: 'vivid-line-grow 0.6s cubic-bezier(0.22, 1, 0.36, 1) 0.4s forwards', transformOrigin: 'center' }}
                />
              </span>
            </h1>
            <p className="vivid-opacity-start vivid-animate-in-up vivid-delay-2 mx-auto mt-8 text-lg leading-relaxed text-fd-muted-foreground md:mt-10 md:text-xl">
              高性能、类型安全的 Go 语言 Actor 模型实现库，提供完整的 Actor
              系统、消息传递、远程通信与监督策略，助你构建可扩展、高并发的分布式应用。
            </p>
            <div className="vivid-opacity-start vivid-animate-in-up vivid-delay-4 mt-12 flex flex-wrap items-center justify-center gap-4 md:mt-14">
              <Link
                href="/docs"
                className={`${buttonVariants({ color: 'primary', size: 'sm' })} vivid-btn-glow gap-2 px-6 py-3 text-base shadow-[0_0_20px_rgba(212,175,55,0.2)]`}
              >
                <BookOpen className="size-4.5" />
                开始阅读文档
              </Link>
              <a
                href="https://pkg.go.dev/github.com/kercylan98/vivid"
                target="_blank"
                rel="noopener noreferrer"
                className={`${buttonVariants({ color: 'secondary', size: 'sm' })} vivid-btn-outline-hover gap-2 px-6 py-3 text-base`}
              >
                <ExternalLink className="size-4.5" />
                API 文档
              </a>
              <a
                href="https://github.com/kercylan98/vivid"
                target="_blank"
                rel="noopener noreferrer"
                className={`${buttonVariants({ color: 'ghost', size: 'sm' })} vivid-btn-outline-hover gap-2 px-6 py-3 text-base`}
              >
                <Github className="size-4.5" />
                GitHub
              </a>
            </div>
          </div>
        </section>

        {/* 快速开始（在核心特性上方） */}
        <section className="py-16 md:py-24">
          <div className="mx-auto w-full">
            <h2 className="vivid-opacity-start vivid-animate-in-up vivid-section-title text-center text-2xl font-semibold tracking-tight md:text-3xl">
              快速开始
            </h2>
            <p className="vivid-opacity-start vivid-animate-in-up vivid-delay-1 mx-auto mt-6 max-w-xl text-center text-fd-muted-foreground md:mt-7">
              使用 Go 模块安装，即可在项目中使用 Vivid
            </p>
            <div className="mx-auto mt-10 max-w-2xl md:mt-12">
              <div className="overflow-hidden rounded-xl border border-fd-border/80 bg-fd-card shadow-lg">
                <div className="flex gap-4 border-b border-fd-border/60 p-4 md:p-5">
                  <span className="flex h-8 w-8 shrink-0 items-center justify-center rounded-full bg-[var(--vivid-gold)]/20 text-sm font-semibold text-[var(--vivid-gold)]">
                    1
                  </span>
                  <div className="min-w-0 flex-1">
                    <p className="mb-2 text-sm font-medium text-fd-foreground">安装依赖</p>
                    <div className="rounded-lg border border-fd-border/60 bg-[var(--vivid-terminal-bg)] px-4 py-3 font-mono text-sm text-fd-muted-foreground">
                      <span className="flex items-center gap-2 text-[var(--vivid-gold-muted)]">
                        <Terminal className="size-4" />
                        <span>$</span>
                      </span>
                      <code className="mt-1 block text-[var(--vivid-gold-soft)]">go get github.com/kercylan98/vivid</code>
                    </div>
                  </div>
                </div>
                <div className="flex gap-4 p-4 md:p-5">
                  <span className="flex h-8 w-8 shrink-0 items-center justify-center rounded-full bg-[var(--vivid-gold)]/20 text-sm font-semibold text-[var(--vivid-gold)]">
                    2
                  </span>
                  <div className="min-w-0 flex-1">
                    <p className="mb-2 text-sm font-medium text-fd-foreground">下一步</p>
                    <p className="text-sm leading-relaxed text-fd-muted-foreground">
                      在代码中创建 Actor 系统与 Actor，并参考{' '}
                      <Link href="/docs" className="font-medium text-[var(--vivid-gold-soft)] underline underline-offset-2 hover:text-[var(--vivid-gold)]">
                        文档
                      </Link>{' '}
                      与{' '}
                      <a
                        href="https://pkg.go.dev/github.com/kercylan98/vivid"
                        target="_blank"
                        rel="noopener noreferrer"
                        className="font-medium text-[var(--vivid-gold-soft)] underline underline-offset-2 hover:text-[var(--vivid-gold)]"
                      >
                        API 文档
                      </a>{' '}
                      进行开发。
                    </p>
                  </div>
                </div>
              </div>
              <p className="mt-3 flex items-center justify-center gap-1.5 text-xs text-fd-muted-foreground">
                <ArrowRight className="size-3.5" />
                更多示例与概念请查看文档
              </p>
            </div>
          </div>
        </section>

        {/* CTA：放在中间（快速开始与核心特性之间） */}
        <section className="py-16 text-center md:py-24">
          <div className="mx-auto w-full">
            <h2 className="text-2xl font-semibold tracking-tight md:text-3xl">
              准备好构建分布式应用了吗？
            </h2>
            <p className="mt-4 text-fd-muted-foreground md:mt-5">
              从文档与示例开始，或到 GitHub 参与贡献
            </p>
            <div className="mt-10 flex flex-wrap justify-center gap-4 md:mt-12">
              <Link
                href="/docs"
                className={`${buttonVariants({ color: 'primary' })} vivid-btn-glow gap-2 px-6 py-3 shadow-[0_0_20px_rgba(212,175,55,0.25)]`}
              >
                <BookOpen className="size-4" />
                阅读文档
              </Link>
              <a
                href="https://github.com/kercylan98/vivid"
                target="_blank"
                rel="noopener noreferrer"
                className={`${buttonVariants({ color: 'outline' })} vivid-btn-outline-hover gap-2 border-[var(--vivid-gold-muted)] px-6 py-3 text-fd-foreground hover:bg-[var(--vivid-gold-subtle)] hover:border-[var(--vivid-gold)]`}
              >
                <Github className="size-4" />
                查看 GitHub
              </a>
            </div>
          </div>
        </section>

        {/* 核心特性 */}
        <section className="py-16 md:py-24">
          <div className="mx-auto w-full">
            <h2 className="vivid-opacity-start vivid-animate-in-up vivid-delay-1 vivid-section-title text-center text-2xl font-semibold tracking-tight md:text-3xl">
              核心特性
            </h2>
            <p className="vivid-opacity-start vivid-animate-in-up vivid-delay-2 mx-auto mt-6 max-w-xl text-center text-fd-muted-foreground md:mt-7">
              Actor 模型完整实现，从本地到分布式一站到位
            </p>
            <div className="mt-10 grid gap-6 sm:grid-cols-2 lg:mt-14 lg:grid-cols-3 lg:gap-8">
              {featureCards.map((item, i) => (
                <div
                  key={item.title}
                  className="vivid-opacity-start vivid-animate-scale-in vivid-card-hover"
                  style={{ animationDelay: `${0.15 * i + 0.2}s` }}
                >
                  <Card
                    icon={<item.icon className="size-4 text-[var(--vivid-gold)]" />}
                    title={item.title}
                    description={item.description}
                    href="/docs/introduction/quickstart"
                    className="h-full border-fd-border/80 hover:border-[var(--vivid-gold-muted)]"
                  />
                </div>
              ))}
            </div>
          </div>
        </section>
      </div>
    </>
  );
}
