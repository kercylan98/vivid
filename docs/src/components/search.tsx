'use client';
import {
  SearchDialog,
  SearchDialogClose,
  SearchDialogContent,
  SearchDialogFooter,
  SearchDialogHeader,
  SearchDialogIcon,
  SearchDialogInput,
  SearchDialogList,
  SearchDialogOverlay,
  type SharedProps,
  TagsList,
  TagsListItem,
} from 'fumadocs-ui/components/dialog/search';
import { useDocsSearch } from 'fumadocs-core/search/client';
import { create } from '@orama/orama';
import { useI18n } from 'fumadocs-ui/contexts/i18n';
import { useState } from 'react';

// https://fumadocs.nodejs.cn/docs/headless/search/orama#tag-filter
function initOrama(locale?: string) {
  return create({
    schema: { _: 'string' },
    language: 'english', // Orama 不支持 chinese，cn 分词由服务端/静态索引已处理好
  });
}

export default function DefaultSearchDialog(props: SharedProps) {
  const { locale } = useI18n(); // (optional) for i18n
  const [tag, setTag] = useState<string | undefined>();
  const basePath = process.env.NEXT_PUBLIC_BASE_PATH ?? '';
  // 开发用 API 路由，生产用构建时生成的静态文件
  const from =
    process.env.NODE_ENV === 'development'
      ? `${basePath}/api/search`
      : `${basePath}/search-index.json`;
  const { search, setSearch, query } = useDocsSearch({
    tag,
    type: 'static',
    from,
    initOrama,
    locale,
  });


  return (
    <SearchDialog search={search} onSearchChange={setSearch} isLoading={query.isLoading} {...props}>
      <SearchDialogOverlay />
      <SearchDialogContent>
        <SearchDialogHeader>
          <SearchDialogIcon />
          <SearchDialogInput />
          <SearchDialogClose />
        </SearchDialogHeader>
        <SearchDialogList items={query.data !== 'empty' ? query.data : null} />
        <SearchDialogFooter className="flex flex-row">
          <TagsList tag={tag} onTagChange={setTag}>
            <TagsListItem value="my-value">My Value</TagsListItem>
          </TagsList>
        </SearchDialogFooter>
      </SearchDialogContent>
    </SearchDialog>
  );
}