import type {
  Community,
  CommunityMember,
  CommunityView,
  ImportCommitRequest,
  ImportPreview,
  ImportResult,
  ImportSupplier,
  Item,
  LoadoutDetail,
  LoadoutEntry,
  LoadoutSummary,
  Profile,
  ProfileItemLayer,
  ResolvedItem,
  SlotDefinition,
  TemplateDetail,
  Visibility,
} from './types';

const BASE_URL = import.meta.env.VITE_API_URL ?? 'http://localhost:8080/api/v1';

/** The profile the client is acting as. Day 0 auth is a single header. */
let actingProfileId = '';

export function setActingProfile(profileId: string) {
  actingProfileId = profileId;
}

export class ApiError extends Error {
  status: number;
  code: string;

  constructor(status: number, code: string, message: string) {
    super(message);
    this.status = status;
    this.code = code;
  }
}

async function request<T>(path: string, init: RequestInit = {}): Promise<T> {
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
    ...((init.headers as Record<string, string>) ?? {}),
  };
  if (actingProfileId) {
    headers['X-Profile-ID'] = actingProfileId;
  }

  let response: Response;
  try {
    response = await fetch(`${BASE_URL}${path}`, { ...init, headers });
  } catch (cause) {
    // Network-level failure: almost always "the Go server isn't running".
    throw new ApiError(0, 'offline', `Cannot reach the Loadouts API at ${BASE_URL}`);
  }

  if (response.status === 204) {
    return undefined as T;
  }

  const text = await response.text();
  const payload = text ? JSON.parse(text) : undefined;

  if (!response.ok) {
    const detail = payload?.error ?? {};
    throw new ApiError(response.status, detail.code ?? 'error', detail.message ?? response.statusText);
  }
  return payload as T;
}

function qs(params: Record<string, string | number | undefined>): string {
  const search = new URLSearchParams();
  Object.entries(params).forEach(([key, value]) => {
    if (value !== undefined && value !== '') search.set(key, String(value));
  });
  const encoded = search.toString();
  return encoded ? `?${encoded}` : '';
}

export const api = {
  health: () => fetch(`${BASE_URL.replace('/api/v1', '')}/healthz`).then((r) => r.ok),

  // --- Identity ---
  listProfiles: () => request<Profile[]>('/profiles'),
  getProfile: (handle: string) => request<Profile>(`/profiles/${handle}`),
  profileLoadouts: (handle: string) => request<LoadoutSummary[]>(`/profiles/${handle}/loadouts`),
  profileCommunities: (handle: string) => request<Community[]>(`/profiles/${handle}/communities`),
  createUser: (body: { email: string; display_name: string; handle: string }) =>
    request<{ user: unknown; profile: Profile }>('/users', { method: 'POST', body: JSON.stringify(body) }),

  // --- Communities ---
  listCommunities: (q?: string) => request<Community[]>(`/communities${qs({ q })}`),
  getCommunity: (slug: string) => request<CommunityView>(`/communities/${slug}`),
  createCommunity: (body: { name: string; slug?: string; description?: string }) =>
    request<Community>('/communities', { method: 'POST', body: JSON.stringify(body) }),
  joinCommunity: (slug: string) => request<unknown>(`/communities/${slug}/join`, { method: 'POST' }),
  leaveCommunity: (slug: string) => request<void>(`/communities/${slug}/leave`, { method: 'POST' }),
  communityMembers: (slug: string) => request<CommunityMember[]>(`/communities/${slug}/members`),
  communityTemplates: (slug: string) => request<TemplateDetail[]>(`/communities/${slug}/templates`),
  communityLoadouts: (slug: string) => request<LoadoutSummary[]>(`/communities/${slug}/loadouts`),
  setCommunityItemLayer: (slug: string, itemId: string, metadata: Record<string, unknown>) =>
    request<unknown>(`/communities/${slug}/items/${itemId}/layer`, {
      method: 'PUT',
      body: JSON.stringify({ metadata }),
    }),

  // --- Items & layers ---
  listItems: (q?: string) => request<Item[]>(`/items${qs({ q })}`),
  getItem: (id: string, opts: { community?: string; owner?: string } = {}) =>
    request<ResolvedItem>(`/items/${id}${qs(opts)}`),
  createItem: (body: Partial<Item>) => request<Item>('/items', { method: 'POST', body: JSON.stringify(body) }),
  getProfileLayer: (itemId: string, owner?: string) =>
    request<ProfileItemLayer>(`/items/${itemId}/layers/profile${qs({ owner })}`),
  setProfileLayer: (
    itemId: string,
    body: {
      custom_image_url?: string;
      public_metadata?: Record<string, unknown>;
      private_metadata?: Record<string, unknown>;
    },
  ) => request<ProfileItemLayer>(`/items/${itemId}/layers/profile`, { method: 'PUT', body: JSON.stringify(body) }),

  // --- Item importing ---
  importSuppliers: () =>
    request<{ suppliers: ImportSupplier[]; note: string }>('/imports/suppliers'),
  /** Read-only: inspects the URL and returns an editable draft without writing anything. */
  previewImport: (url: string) =>
    request<ImportPreview>('/imports/preview', { method: 'POST', body: JSON.stringify({ url }) }),
  commitImport: (body: ImportCommitRequest) =>
    request<ImportResult>('/imports/commit', { method: 'POST', body: JSON.stringify(body) }),

  // --- Templates ---
  listTemplates: (params: { q?: string; community_id?: string } = {}) =>
    request<TemplateDetail[]>(`/templates${qs(params)}`),
  getTemplate: (id: string, version?: number) => request<TemplateDetail>(`/templates/${id}${qs({ version })}`),
  createTemplate: (body: {
    name: string;
    description?: string;
    owner_type?: string;
    community_id?: string;
    is_public?: boolean;
    slots: SlotDefinition[];
  }) => request<TemplateDetail>('/templates', { method: 'POST', body: JSON.stringify(body) }),
  publishTemplateVersion: (id: string, body: { slots: SlotDefinition[]; changelog?: string }) =>
    request<TemplateDetail>(`/templates/${id}/versions`, { method: 'POST', body: JSON.stringify(body) }),

  // --- Loadouts ---
  myLoadouts: () => request<LoadoutSummary[]>('/loadouts'),
  discover: (params: { q?: string; community_id?: string } = {}) =>
    request<LoadoutSummary[]>(`/discover${qs(params)}`),
  getLoadout: (id: string) => request<LoadoutDetail>(`/loadouts/${id}`),
  createLoadout: (body: {
    name: string;
    description?: string;
    template_id?: string;
    community_id?: string;
    visibility?: Visibility;
  }) => request<LoadoutDetail>('/loadouts', { method: 'POST', body: JSON.stringify(body) }),
  updateLoadout: (id: string, body: Record<string, unknown>) =>
    request<LoadoutDetail>(`/loadouts/${id}`, { method: 'PATCH', body: JSON.stringify(body) }),
  replaceEntries: (id: string, entries: LoadoutEntry[]) =>
    request<LoadoutDetail>(`/loadouts/${id}/entries`, { method: 'PUT', body: JSON.stringify({ entries }) }),
  publishLoadout: (id: string, body: { visibility?: Visibility; community_id?: string } = {}) =>
    request<LoadoutDetail>(`/loadouts/${id}/publish`, { method: 'POST', body: JSON.stringify(body) }),
  forkLoadout: (id: string) => request<LoadoutDetail>(`/loadouts/${id}/fork`, { method: 'POST' }),
  deleteLoadout: (id: string) => request<void>(`/loadouts/${id}`, { method: 'DELETE' }),
};
