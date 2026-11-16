'use client';

import React, { useMemo, useState } from 'react';
import type { MarketplaceRouteItem } from '@/types/marketplace';
import {
  MarketplaceFilterBar,
  MarketplaceGrid,
  MarketplaceHero,
  MarketplaceIntro,
  MarketplaceQuickPreview,
  MarketplaceValueSection,
} from './marketplace';

interface MarketplaceExplorerProps {
  initialRoutes: MarketplaceRouteItem[];
}

export function MarketplaceExplorer({ initialRoutes }: MarketplaceExplorerProps) {
  const [searchQuery, setSearchQuery] = useState('');
  const [selectedCategory, setSelectedCategory] = useState<string>('all');
  const [selectedCluster, setSelectedCluster] = useState<string>('all');
  const [activeQuickPreviewSlug, setActiveQuickPreviewSlug] = useState<string | null>(null);
  const [quickSdkLang, setQuickSdkLang] = useState<'python' | 'typescript'>('python');

  // Filter routes by category, cluster, and search query
  const filteredRoutes = useMemo(() => {
    return initialRoutes.filter((route) => {
      if (
        selectedCategory !== 'all' &&
        route.category.toLowerCase() !== selectedCategory.toLowerCase()
      ) {
        return false;
      }
      if (selectedCluster !== 'all') {
        const cluster = (route.cluster || '').toLowerCase();
        if (!cluster.includes(selectedCluster.toLowerCase())) return false;
      }
      if (searchQuery.trim() !== '') {
        const q = searchQuery.trim().toLowerCase();
        const haystack = [
          route.title,
          route.slug,
          route.summary || '',
          route.category,
          route.path_pattern,
          route.provider_name,
          route.model_name || '',
          ...route.tags,
        ]
          .join(' ')
          .toLowerCase();
        if (!haystack.includes(q)) return false;
      }
      return true;
    });
  }, [initialRoutes, searchQuery, selectedCategory, selectedCluster]);

  const previewRoute = useMemo(() => {
    if (!activeQuickPreviewSlug) return null;
    return initialRoutes.find((r) => r.slug === activeQuickPreviewSlug) || null;
  }, [initialRoutes, activeQuickPreviewSlug]);

  const handleResetFilters = () => {
    setSearchQuery('');
    setSelectedCategory('all');
    setSelectedCluster('all');
  };

  const handleOpenQuickSdk = (slug: string) => {
    setActiveQuickPreviewSlug((prev) => (prev === slug ? null : slug));
  };

  return (
    <div className="space-y-12">
      {/* 1. Hero Banner with Instant Search */}
      <MarketplaceHero
        searchQuery={searchQuery}
        onSearchChange={setSearchQuery}
      />

      {/* 2. Platform Value, 3 Pillars, and Quick Verification */}
      <MarketplaceIntro />

      {/* 3. Category & Settlement Network Filters */}
      <MarketplaceFilterBar
        initialRoutes={initialRoutes}
        filteredCount={filteredRoutes.length}
        selectedCategory={selectedCategory}
        selectedCluster={selectedCluster}
        onSelectCategory={setSelectedCategory}
        onSelectCluster={setSelectedCluster}
      />

      {/* 4. Quick SDK Drawer / Modal (if triggered from card) */}
      {previewRoute && (
        <MarketplaceQuickPreview
          previewRoute={previewRoute}
          sdkLang={quickSdkLang}
          onLangChange={setQuickSdkLang}
          onClose={() => setActiveQuickPreviewSlug(null)}
        />
      )}

      {/* 5. Route Cards Grid (Protected against wrapping and layout distortion) */}
      <MarketplaceGrid
        routes={filteredRoutes}
        searchQuery={searchQuery}
        onResetFilters={handleResetFilters}
        onOpenQuickSdk={handleOpenQuickSdk}
      />

      {/* 6. Dual-layer Architecture & /llms.txt Value Section */}
      <MarketplaceValueSection />
    </div>
  );
}
