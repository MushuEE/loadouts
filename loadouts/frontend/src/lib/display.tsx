import {
  Activity,
  Box,
  Droplets,
  Flashlight,
  Footprints,
  Layers,
  PackageOpen,
  Shirt,
  ShoppingBag,
  Tent,
  Utensils,
} from 'lucide-react';
import type { LayerName, ResolvedItem } from '../api/types';

/** Core metadata namespace shared by every hobby (see backend core.CoreNamespace). */
export const CORE = 'core';

export function weightG(item: ResolvedItem | undefined): number {
  return Number(item?.metadata?.[CORE]?.weight_g ?? 0);
}

export function costCents(item: ResolvedItem | undefined): number {
  return Number(item?.metadata?.[CORE]?.cost_cents ?? 0);
}

export function isConsumable(item: ResolvedItem | undefined): boolean {
  return Boolean(item?.metadata?.[CORE]?.consumable);
}

export function formatKg(grams: number): string {
  return `${(grams / 1000).toFixed(2)} kg`;
}

export function formatGrams(grams: number): string {
  return `${Math.round(grams)}g`;
}

export function formatCost(cents: number): string {
  return `$${(cents / 100).toLocaleString(undefined, { maximumFractionDigits: 0 })}`;
}

/** Visual identity per layer, used by the provenance chips in the item inspector. */
export const LAYER_STYLES: Record<LayerName, { label: string; className: string }> = {
  global: { label: 'Global', className: 'bg-stone-700/60 text-stone-300' },
  community: { label: 'Community', className: 'bg-sky-500/20 text-sky-300' },
  user_public: { label: 'You (public)', className: 'bg-emerald-500/20 text-emerald-300' },
  user_private: { label: 'You (private)', className: 'bg-purple-500/20 text-purple-300' },
};

export function categoryIcon(category: string, className = 'w-5 h-5') {
  switch (category) {
    case 'pack':
      return <ShoppingBag className={className} />;
    case 'shelter':
      return <Tent className={className} />;
    case 'consumable':
    case 'fuel':
      return <Droplets className={className} />;
    case 'kitchen':
      return <Utensils className={className} />;
    case 'electronics':
      return <Flashlight className={className} />;
    case 'shirt':
    case 'outerwear':
      return <Shirt className={className} />;
    case 'pants':
    case 'sleep':
      return <Layers className={className} />;
    case 'shoes':
      return <Footprints className={className} />;
    case 'poles':
      return <Activity className={className} />;
    case 'organizer':
      return <PackageOpen className={className} />;
    default:
      return <Box className={className} />;
  }
}
