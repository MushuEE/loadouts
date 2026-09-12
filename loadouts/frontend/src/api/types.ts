// Types mirroring the Go API (see backend/internal/core).

export interface Profile {
  id: string;
  user_id: string;
  handle: string;
  display_name: string;
  bio: string;
  avatar_url: string;
  is_sponsor: boolean;
  created_at: string;
}

export interface User {
  id: string;
  email: string;
  display_name: string;
  created_at: string;
}

export type MemberRole = 'member' | 'admin' | 'owner' | '';

export interface Community {
  id: string;
  slug: string;
  name: string;
  description: string;
  created_by: string;
  member_count: number;
  metadata_hint: Record<string, unknown>;
  created_at: string;
}

export interface CommunityView {
  community: Community;
  viewer_role: MemberRole;
  member_count: number;
}

export interface CommunityMember {
  membership: { community_id: string; profile_id: string; role: MemberRole; joined_at: string };
  profile: Profile;
}

export interface SlotDefinition {
  id: string;
  name: string;
  description?: string;
  accepted_categories: string[];
  required: boolean;
  max_items: number;
  position: number;
}

/** Which layer a resolved metadata value came from. */
export type LayerName = 'global' | 'community' | 'user_public' | 'user_private';

export interface ResolvedItem {
  id: string;
  name: string;
  category: string;
  image_url: string;
  provided_slots: SlotDefinition[];
  metadata: Record<string, Record<string, unknown>>;
  provenance: Record<string, LayerName>;
  applied_layers: LayerName[];
  sources?: { supplier_name: string; price: number; url: string }[];
  origin?: ItemOrigin;
  verified: boolean;
}

/** How an item entered the catalog. Imports are usable but flagged until vouched for. */
export type ItemOrigin = 'curated' | 'import';

export interface Item {
  id: string;
  name: string;
  category: string;
  image_url: string;
  provided_slots: SlotDefinition[];
  base_metadata: Record<string, Record<string, unknown>>;
  origin?: ItemOrigin;
  verified?: boolean;
  imported_by?: string;
}

// --- Item importing ---

/** Where the URL pointed, derived without any network call. */
export interface ImportTarget {
  supplier_id: string;
  product_id: string;
  canonical_url: string;
  host: string;
}

/**
 * The scraped draft. `has_weight` / `has_price` distinguish "we couldn't find it" from
 * "it is genuinely zero", which matters because 0g would silently corrupt loadout stats.
 */
export interface ImportDraft {
  target: ImportTarget;
  name: string;
  brand: string;
  description: string;
  image_url: string;
  category: string;
  weight_g: number;
  has_weight: boolean;
  cost_cents: number;
  has_price: boolean;
  currency: string;
  consumable: boolean;
  extras?: Record<string, unknown>;
  extracted_via?: string;
}

export type ImportStatus = 'parsed' | 'manual' | 'existing';

export interface ImportPreview {
  status: ImportStatus;
  draft: ImportDraft;
  supplier_name: string;
  affiliate_url: string;
  suggested_id: string;
  existing_item?: Item;
  warning?: string;
}

export interface ImportCommitRequest {
  url: string;
  name: string;
  category: string;
  brand?: string;
  description?: string;
  image_url?: string;
  weight_g: number;
  cost_cents: number;
  currency?: string;
  consumable?: boolean;
}

export interface ImportResult {
  item: Item;
  /** False when the URL turned out to already be in the catalog. */
  created: boolean;
}

export interface ImportSupplier {
  id: string;
  name: string;
  base_url: string;
}

export interface ProfileItemLayer {
  profile_id: string;
  item_id: string;
  custom_image_url: string;
  public_metadata: Record<string, Record<string, unknown>>;
  private_metadata: Record<string, Record<string, unknown>>;
  updated_at: string;
}

export type OwnerType = 'platform' | 'profile' | 'community';

export interface Template {
  id: string;
  name: string;
  description: string;
  owner_type: OwnerType;
  owner_id: string;
  community_id: string;
  latest_version: number;
  is_public: boolean;
}

export interface TemplateVersion {
  template_id: string;
  version: number;
  slots: SlotDefinition[];
  changelog: string;
  created_at: string;
}

export interface TemplateDetail {
  template: Template;
  version: TemplateVersion;
  versions?: number[];
}

export type Visibility = 'private' | 'unlisted' | 'public';
export type LoadoutStatus = 'draft' | 'published';

export interface Loadout {
  id: string;
  name: string;
  description: string;
  owner_profile_id: string;
  community_id: string;
  template_id: string;
  template_version: number;
  visibility: Visibility;
  status: LoadoutStatus;
  forked_from: string;
  cover_image_url: string;
  fork_count: number;
  created_at: string;
  updated_at: string;
}

export interface LoadoutEntry {
  id: string;
  loadout_id?: string;
  slot_id: string;
  parent_entry_id: string;
  item_id: string;
  quantity: number;
  note: string;
  position: number;
}

export interface ResolvedEntry {
  entry: LoadoutEntry;
  item: ResolvedItem;
  children?: ResolvedEntry[];
}

export interface LoadoutStats {
  total_weight_g: number;
  base_weight_g: number;
  consumable_weight_g: number;
  total_cost_cents: number;
  item_count: number;
}

export interface ValidationIssue {
  slot_id: string;
  entry_id?: string;
  severity: 'error' | 'warning';
  message: string;
}

export interface LoadoutDetail {
  loadout: Loadout;
  owner: Profile;
  template: TemplateDetail;
  entries: ResolvedEntry[];
  stats: LoadoutStats;
  issues: ValidationIssue[];
}

export interface LoadoutSummary {
  loadout: Loadout;
  owner_handle: string;
  owner_name: string;
  template_name: string;
  community_id?: string;
  stats: LoadoutStats;
  item_preview: string[];
}

// --- Plugins ---
//
// A plugin extends the UI with charts, calculators, tables, and maps. There are two
// execution tiers, and the difference matters to this client: a `widget` arrives fully
// evaluated by the server (literal values only, no expressions), while an `embed` is
// author HTML we load into a sandboxed iframe and talk to over postMessage.

export type PluginSurface = 'loadout.panel' | 'loadout.sidebar' | 'item.tab' | 'community.tab';
export type PluginViewKind = 'widget' | 'embed';
export type WidgetType = 'stat_grid' | 'bar_chart' | 'pie_chart' | 'table';

export interface PluginCapabilities {
  read_loadout: boolean;
  read_items: boolean;
  read_community: boolean;
  storage: boolean;
  network?: string[];
}

export interface PluginSettingDefinition {
  key: string;
  label: string;
  type: 'text' | 'secret' | 'number' | 'bool' | 'select';
  required: boolean;
  default?: string;
  help?: string;
  options?: string[];
}

export interface PluginManifest {
  api_version: number;
  views: PluginViewDefinition[];
  capabilities: PluginCapabilities;
  schemas?: { namespace: string; definition: unknown }[];
  settings?: PluginSettingDefinition[];
}

export interface PluginViewDefinition {
  id: string;
  title: string;
  surface: PluginSurface;
  kind: PluginViewKind;
  height?: number;
}

export interface Plugin {
  id: string;
  slug: string;
  name: string;
  description: string;
  owner_type: 'platform' | 'profile' | 'community';
  owner_id: string;
  latest_version: number;
  is_public: boolean;
  created_at: string;
  updated_at: string;
}

export interface PluginVersion {
  plugin_id: string;
  version: number;
  manifest: PluginManifest;
  changelog: string;
  created_by: string;
  created_at: string;
}

export interface PluginDetail {
  plugin: Plugin;
  version: PluginVersion;
}

export interface PluginInstall {
  id: string;
  plugin_id: string;
  version: number;
  scope_type: 'profile' | 'community';
  scope_id: string;
  granted_caps: string[];
  settings: Record<string, unknown>;
  enabled: boolean;
  installed_by: string;
  created_at: string;
  updated_at: string;
}

export interface PluginInstallView {
  install: PluginInstall;
  plugin: Plugin;
  manifest: PluginManifest;
  upgrade_available: boolean;
  /** Non-empty means upgrading would require approving more access. */
  missing_caps?: string[];
}

// --- Evaluated widget render model ---

export interface RenderedStat {
  label: string;
  value: unknown;
  display: string;
  unit?: string;
  help?: string;
}

export interface RenderedColumn {
  label: string;
  align: string;
}

export interface RenderedCell {
  value: unknown;
  display: string;
}

export interface RenderedPoint {
  label: string;
  value: number;
  display: string;
  share: number;
}

export interface WidgetRender {
  type: WidgetType;
  empty: boolean;
  empty_text?: string;
  stats?: RenderedStat[];
  columns?: RenderedColumn[];
  rows?: RenderedCell[][];
  points?: RenderedPoint[];
  total?: number;
  truncated?: boolean;
}

export interface RenderedView {
  install_id: string;
  plugin_id: string;
  version: number;
  view_id: string;
  title: string;
  surface: PluginSurface;
  kind: PluginViewKind;
  height?: number;
  widget?: WidgetRender;
  embed_url?: string;
  embed_context?: Record<string, unknown>;
  /** A view that failed to render. One broken plugin degrades to its own card. */
  error?: string;
}

export interface PluginDatum {
  plugin_id: string;
  scope_type: string;
  scope_id: string;
  key: string;
  value: Record<string, unknown>;
  updated_by: string;
  updated_at: string;
}
