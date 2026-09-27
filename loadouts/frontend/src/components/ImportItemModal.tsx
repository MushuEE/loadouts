import { useEffect, useState } from 'react';
import { AlertTriangle, CheckCircle2, ClipboardPaste, ExternalLink, Link2, Loader2, PackagePlus, X } from 'lucide-react';
import { api, ApiError } from '../api/client';
import type { ImportDraft, ImportPreview, Item } from '../api/types';
import { formatCost, formatGrams } from '../lib/display';
import { ErrorNote } from './ui';

const CATEGORIES = [
  'pack', 'shelter', 'sleep', 'kitchen', 'fuel', 'water', 'shoes',
  'shirt', 'jacket', 'pants', 'poles', 'electronics', 'consumable', 'other',
];

/** The editable fields, kept separate from the preview so user edits survive a re-preview. */
interface FormState {
  name: string;
  category: string;
  brand: string;
  imageUrl: string;
  /** Kept as strings so the inputs can be genuinely empty rather than showing a stray 0. */
  weightG: string;
  costDollars: string;
  consumable: boolean;
  description: string;
  /** Scraped details with no dedicated field (material, color, sku...). Clearing one drops it. */
  extras: Record<string, string>;
}

const EMPTY_FORM: FormState = {
  name: '', category: '', brand: '', imageUrl: '', weightG: '', costDollars: '', consumable: false,
  description: '', extras: {},
};

function formFromDraft(d: ImportDraft): FormState {
  return {
    name: d.name,
    category: d.category,
    brand: d.brand,
    imageUrl: d.image_url,
    // Only prefill when the page actually told us; a blank field prompts the user,
    // whereas a prefilled 0 quietly becomes a weightless item.
    weightG: d.has_weight ? String(Math.round(d.weight_g)) : '',
    costDollars: d.has_price ? (d.cost_cents / 100).toFixed(2) : '',
    consumable: d.consumable,
    description: d.description,
    extras: Object.fromEntries(Object.entries(d.extras ?? {}).map(([k, v]) => [k, String(v)])),
  };
}

/**
 * Paste-a-URL item import.
 *
 * Two steps on purpose: preview reads the page and prefills, the user corrects anything
 * wrong, then commit writes it. Scraped data is never good enough to trust straight into
 * a shared catalog, and weight in particular has to be right or it silently corrupts
 * every loadout the item lands in.
 */
export function ImportItemModal({
  onClose,
  onImported,
}: {
  onClose: () => void;
  onImported: (item: Item) => void;
}) {
  const [url, setUrl] = useState('');
  const [preview, setPreview] = useState<ImportPreview | null>(null);
  const [form, setForm] = useState<FormState>(EMPTY_FORM);
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState('');
  const [pasteOpen, setPasteOpen] = useState(false);
  const [pastedHTML, setPastedHTML] = useState('');
  const [supportedNames, setSupportedNames] = useState<string[]>([]);

  useEffect(() => {
    api
      .importSuppliers()
      .then((r) => setSupportedNames(r.suppliers.map((s) => s.name)))
      .catch(() => setSupportedNames([]));
  }, []);

  async function runPreview(html?: string) {
    if (!url.trim()) return;
    setLoading(true);
    setError('');
    setPreview(null);
    try {
      const result = await api.previewImport(url.trim(), html);
      setPreview(result);
      setForm(formFromDraft(result.draft));
      if (html) {
        setPasteOpen(false);
        setPastedHTML('');
      }
    } catch (e) {
      setError(e instanceof ApiError ? e.message : String(e));
    } finally {
      setLoading(false);
    }
  }

  async function commit() {
    if (!preview) return;
    setSaving(true);
    setError('');
    try {
      const result = await api.commitImport({
        url: preview.draft.target.canonical_url,
        name: form.name.trim(),
        category: form.category,
        brand: form.brand,
        image_url: form.imageUrl,
        weight_g: Number(form.weightG) || 0,
        cost_cents: Math.round((Number(form.costDollars) || 0) * 100),
        currency: preview.draft.currency || 'USD',
        consumable: form.consumable,
        description: form.description.trim(),
        extras: form.extras,
      });
      onImported(result.item);
      onClose();
    } catch (e) {
      setError(e instanceof ApiError ? e.message : String(e));
    } finally {
      setSaving(false);
    }
  }

  const existing = preview?.status === 'existing' ? preview.existing_item : undefined;
  const canSave = !!preview && !existing && form.name.trim().length > 0;

  return (
    <div className="fixed inset-0 z-50 bg-black/70 backdrop-blur-sm flex items-center justify-center p-4">
      <div className="bg-stone-900 border border-stone-700 rounded-xl w-full max-w-lg max-h-[88vh] flex flex-col">
        <div className="p-4 border-b border-stone-800 flex justify-between items-start">
          <div>
            <h3 className="font-bold text-white flex items-center gap-2">
              <PackagePlus className="w-4 h-4 text-orange-500" />
              Import gear from a store
            </h3>
            <p className="text-[11px] text-stone-500 mt-0.5">
              {supportedNames.length > 0
                ? `Recognizes ${supportedNames.join(', ')} — other stores work too.`
                : 'Paste a product link from any store.'}
            </p>
          </div>
          <button onClick={onClose} className="p-1 text-stone-500 hover:text-white">
            <X className="w-4 h-4" />
          </button>
        </div>

        <div className="flex-1 overflow-y-auto p-4 space-y-4">
          <div>
            <label className="block text-[10px] uppercase tracking-wider text-stone-500 mb-1">
              Product URL
            </label>
            <div className="flex gap-2">
              <div className="flex items-center bg-black/40 border border-stone-800 rounded-lg px-3 flex-1 focus-within:border-orange-500">
                <Link2 className="w-4 h-4 text-stone-500 shrink-0" />
                <input
                  autoFocus
                  value={url}
                  onChange={(e) => setUrl(e.target.value)}
                  onKeyDown={(e) => e.key === 'Enter' && runPreview()}
                  placeholder="https://www.rei.com/product/894303/..."
                  className="bg-transparent px-3 py-2 text-sm text-white outline-none flex-1 placeholder:text-stone-600"
                />
              </div>
              <button
                onClick={() => runPreview()}
                disabled={loading || !url.trim()}
                className="px-4 py-2 bg-orange-600 hover:bg-orange-500 disabled:bg-stone-800 disabled:text-stone-600 text-white text-sm font-medium rounded-lg flex items-center gap-2"
              >
                {loading ? <Loader2 className="w-4 h-4 animate-spin" /> : 'Look up'}
              </button>
            </div>
          </div>

          {error && <ErrorNote message={error} />}

          {existing && (
            <div className="p-3 bg-sky-950/40 border border-sky-900 rounded-lg">
              <p className="text-sky-300 text-sm font-medium flex items-center gap-2">
                <CheckCircle2 className="w-4 h-4" />
                Already in the catalog
              </p>
              <p className="text-xs text-stone-400 mt-1">
                “{existing.name}” was imported from this product already. Use it directly instead of
                creating a duplicate.
              </p>
              <button
                onClick={() => {
                  onImported(existing);
                  onClose();
                }}
                className="mt-2 px-3 py-1.5 bg-sky-700 hover:bg-sky-600 text-white text-xs font-medium rounded-md"
              >
                Use this item
              </button>
            </div>
          )}

          {preview && !existing && (
            <>
              {preview.warning && (
                <div className="p-3 bg-amber-950/40 border border-amber-900/60 rounded-lg flex gap-2">
                  <AlertTriangle className="w-4 h-4 text-amber-500 shrink-0 mt-0.5" />
                  <p className="text-xs text-amber-200/90">{preview.warning}</p>
                </div>
              )}

              {preview.status === 'manual' && (
                <div className="p-3 bg-black/30 border border-stone-800 rounded-lg space-y-2">
                  {!pasteOpen ? (
                    <button
                      onClick={() => setPasteOpen(true)}
                      className="text-xs text-stone-300 hover:text-orange-400 flex items-center gap-1.5"
                    >
                      <ClipboardPaste className="w-3.5 h-3.5" />
                      Paste the page source instead
                    </button>
                  ) : (
                    <>
                      <p className="text-[11px] text-stone-500 leading-relaxed">
                        Some stores block automated readers. Open the product in your browser, press{' '}
                        <kbd className="px-1 bg-stone-800 rounded">Ctrl+U</kbd> (
                        <kbd className="px-1 bg-stone-800 rounded">⌥⌘U</kbd> on Mac), select all, copy, and paste it
                        here.
                      </p>
                      <textarea
                        value={pastedHTML}
                        onChange={(e) => setPastedHTML(e.target.value)}
                        rows={4}
                        placeholder="<!DOCTYPE html>..."
                        className="w-full bg-black/40 border border-stone-800 rounded-lg px-3 py-2 text-[11px] text-stone-300 font-mono outline-none focus:border-orange-500 placeholder:text-stone-600"
                      />
                      <button
                        onClick={() => runPreview(pastedHTML)}
                        disabled={loading || !pastedHTML.trim()}
                        className="px-3 py-1.5 bg-stone-700 hover:bg-stone-600 disabled:bg-stone-800 disabled:text-stone-600 text-white text-xs font-medium rounded-md flex items-center gap-2"
                      >
                        {loading && <Loader2 className="w-3 h-3 animate-spin" />}
                        Read pasted page
                      </button>
                    </>
                  )}
                </div>
              )}

              <div className="flex items-center justify-between text-[11px] text-stone-500 border-y border-stone-800 py-2">
                <span>
                  Store: <span className="text-stone-300">{preview.supplier_name}</span>
                </span>
                <a
                  href={preview.affiliate_url}
                  target="_blank"
                  rel="noreferrer"
                  className="flex items-center gap-1 text-stone-400 hover:text-orange-400"
                >
                  View product <ExternalLink className="w-3 h-3" />
                </a>
              </div>

              <div className="flex gap-3">
                {form.imageUrl && (
                  <img
                    src={form.imageUrl}
                    alt=""
                    className="w-20 h-20 object-cover rounded-lg border border-stone-800 bg-black/40"
                    onError={(e) => ((e.target as HTMLImageElement).style.display = 'none')}
                  />
                )}
                <div className="flex-1 space-y-3">
                  <Field label="Name" required>
                    <input
                      value={form.name}
                      onChange={(e) => setForm({ ...form, name: e.target.value })}
                      placeholder="What is it called?"
                      className="w-full bg-black/40 border border-stone-800 rounded-lg px-3 py-2 text-sm text-white outline-none focus:border-orange-500 placeholder:text-stone-600"
                    />
                  </Field>
                  <Field label="Category">
                    <select
                      value={form.category}
                      onChange={(e) => setForm({ ...form, category: e.target.value })}
                      className="w-full bg-black/40 border border-stone-800 rounded-lg px-3 py-2 text-sm text-white outline-none focus:border-orange-500"
                    >
                      <option value="">Uncategorized</option>
                      {CATEGORIES.map((c) => (
                        <option key={c} value={c}>
                          {c}
                        </option>
                      ))}
                    </select>
                  </Field>
                </div>
              </div>

              <div className="grid grid-cols-2 gap-3">
                <Field label="Weight (g)" hint={!preview.draft.has_weight ? 'not found on page' : undefined}>
                  <input
                    type="number"
                    min="0"
                    value={form.weightG}
                    onChange={(e) => setForm({ ...form, weightG: e.target.value })}
                    placeholder="0"
                    className="w-full bg-black/40 border border-stone-800 rounded-lg px-3 py-2 text-sm text-white font-mono outline-none focus:border-orange-500 placeholder:text-stone-600"
                  />
                </Field>
                <Field label="Price" hint={!preview.draft.has_price ? 'not found on page' : undefined}>
                  <input
                    type="number"
                    min="0"
                    step="0.01"
                    value={form.costDollars}
                    onChange={(e) => setForm({ ...form, costDollars: e.target.value })}
                    placeholder="0.00"
                    className="w-full bg-black/40 border border-stone-800 rounded-lg px-3 py-2 text-sm text-white font-mono outline-none focus:border-orange-500 placeholder:text-stone-600"
                  />
                </Field>
              </div>

              <Field label="Description">
                <textarea
                  value={form.description}
                  onChange={(e) => setForm({ ...form, description: e.target.value })}
                  rows={2}
                  className="w-full bg-black/40 border border-stone-800 rounded-lg px-3 py-2 text-xs text-stone-300 outline-none focus:border-orange-500"
                />
              </Field>

              {Object.keys(form.extras).length > 0 && (
                <Field label="Other details" hint="clear a value to leave it out">
                  <div className="space-y-1.5">
                    {Object.entries(form.extras).map(([key, value]) => (
                      <div key={key} className="flex items-center gap-2">
                        <span className="w-28 shrink-0 text-[11px] text-stone-500 truncate">{key.replace(/_/g, ' ')}</span>
                        <input
                          value={value}
                          onChange={(e) => setForm({ ...form, extras: { ...form.extras, [key]: e.target.value } })}
                          className="flex-1 bg-black/40 border border-stone-800 rounded-md px-2 py-1 text-xs text-stone-300 outline-none focus:border-orange-500"
                        />
                      </div>
                    ))}
                  </div>
                </Field>
              )}

              <label className="flex items-center gap-2 text-xs text-stone-400 cursor-pointer">
                <input
                  type="checkbox"
                  checked={form.consumable}
                  onChange={(e) => setForm({ ...form, consumable: e.target.checked })}
                  className="accent-orange-600"
                />
                Consumable (food, fuel, water — excluded from base weight)
              </label>

              {(form.weightG || form.costDollars) && (
                <p className="text-[11px] text-stone-600 font-mono">
                  Will add as {formatGrams(Number(form.weightG) || 0)} ·{' '}
                  {formatCost(Math.round((Number(form.costDollars) || 0) * 100))}
                </p>
              )}
            </>
          )}
        </div>

        <div className="p-4 border-t border-stone-800 flex items-center justify-between gap-3">
          <p className="text-[10px] text-stone-600 flex-1">
            {preview && !existing
              ? 'Imported gear is added to the shared catalog, flagged unverified.'
              : ''}
          </p>
          <button onClick={onClose} className="px-3 py-2 text-sm text-stone-400 hover:text-white">
            Cancel
          </button>
          <button
            onClick={commit}
            disabled={!canSave || saving}
            className="px-4 py-2 bg-orange-600 hover:bg-orange-500 disabled:bg-stone-800 disabled:text-stone-600 text-white text-sm font-medium rounded-lg flex items-center gap-2"
          >
            {saving && <Loader2 className="w-4 h-4 animate-spin" />}
            Add to catalog
          </button>
        </div>
      </div>
    </div>
  );
}

function Field({
  label,
  hint,
  required,
  children,
}: {
  label: string;
  hint?: string;
  required?: boolean;
  children: React.ReactNode;
}) {
  return (
    <div>
      <label className="flex items-baseline justify-between text-[10px] uppercase tracking-wider text-stone-500 mb-1">
        <span>
          {label}
          {required && <span className="text-orange-500 ml-0.5">*</span>}
        </span>
        {hint && <span className="text-amber-600/80 normal-case tracking-normal">{hint}</span>}
      </label>
      {children}
    </div>
  );
}
