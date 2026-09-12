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
}

export interface Item {
  id: string;
  name: string;
  category: string;
  image_url: string;
  provided_slots: SlotDefinition[];
  base_metadata: Record<string, Record<string, unknown>>;
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
