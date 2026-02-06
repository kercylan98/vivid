import type { BaseLayoutProps } from 'fumadocs-ui/layouts/shared';

export function baseOptions(): BaseLayoutProps {
  return {
    nav: {
      title: 'Vivid',
      url: '/',
    },
    links: [
      { text: '文档', url: '/docs' },
    ],
    githubUrl: 'https://github.com/kercylan98/vivid',
  };
}
