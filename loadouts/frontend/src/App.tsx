import React, { useState, useMemo, useEffect } from 'react';
import { 
  ChevronRight, 
  Search, 
  Trash2, 
  Maximize2, 
  ArrowUpLeft,
  Mountain,
  Tent,
  Droplets,
  Utensils,
  Flashlight,
  PackageOpen,
  ShoppingBag,
  Shirt,
  Footprints,
  Activity,
  Layers,
  GripHorizontal,
  Box,
  Plus,
  RefreshCw,
  Home,
  Compass,
  Database,
  Settings,
  Backpack,
  Link as LinkIcon
} from 'lucide-react';

/**
 * --- TYPES ---
 */

interface SlotDefinition {
  id: string;
  name: string;
  acceptedCategories: string[]; 
  capacityLiters?: number;
}

interface Item {
  id: string;
  name: string;
  category: string;
  weight: number;   // grams
  cost: number;     // cents
  consumable: boolean;
  description?: string;
  providedSlots: SlotDefinition[];
}

interface LoadoutNode {
  instanceId: string;
  item: Item;
  children: Record<string, LoadoutNode | null>; // SlotDefinitionID -> Node
  layout: Record<string, number>; // SlotDefinitionID -> Grid Index (0-31)
}

/**
 * --- MOCK DATA ---
 */

const MOCK_DB: Item[] = [
  // --- PACKS ---
  {
    id: 'pack-hgm-3400',
    name: 'Hyperlite 3400 Southwest',
    category: 'pack',
    weight: 910,
    cost: 37500,
    consumable: false,
    providedSlots: [
      { id: 'main', name: 'Main Compartment', acceptedCategories: ['universal'] },
      { id: 'side-left', name: 'Left Side Pocket', acceptedCategories: ['bottle', 'filter', 'shelter'] },
      { id: 'side-right', name: 'Right Side Pocket', acceptedCategories: ['bottle', 'filter', 'shelter'] },
      { id: 'hip-left', name: 'Hip Belt (L)', acceptedCategories: ['electronics', 'snack', 'tool'] },
      { id: 'hip-right', name: 'Hip Belt (R)', acceptedCategories: ['electronics', 'snack', 'tool'] },
      { id: 'front-mesh', name: 'Front Mesh', acceptedCategories: ['rain_gear', 'trowel', 'garbage'] },
    ]
  },
  // --- WORN GEAR ---
  { id: 'shirt-sunhoody', name: 'Patagonia Capilene Cool Daily', category: 'shirt', weight: 153, cost: 5500, consumable: false, providedSlots: [] },
  { id: 'pants-strider', name: 'Patagonia Strider Pro Shorts', category: 'pants', weight: 105, cost: 7900, consumable: false, providedSlots: [] },
  { id: 'shoes-lonepeak', name: 'Altra Lone Peak 8', category: 'shoes', weight: 600, cost: 14000, consumable: false, providedSlots: [] },
  { id: 'poles-bj', name: 'Black Diamond Alpine Carbon Cork', category: 'poles', weight: 480, cost: 19000, consumable: false, providedSlots: [] },
  // --- SHELTER ---
  { id: 'tent-xmid', name: 'Durston X-Mid 1', category: 'shelter', weight: 795, cost: 24000, consumable: false, providedSlots: [] },
  { id: 'stakes-groundhog', name: 'MSR Mini Groundhogs (x6)', category: 'stakes', weight: 60, cost: 2500, consumable: false, providedSlots: [] },
  // --- SLEEP ---
  { id: 'quilt-ee', name: 'Enigma 20° Quilt', category: 'sleep', weight: 550, cost: 35000, consumable: false, providedSlots: [] },
  { id: 'pad-xlight', name: 'Therm-a-Rest UberLite', category: 'sleep', weight: 250, cost: 18000, consumable: false, providedSlots: [] },
  // --- KITCHEN & CONSUMABLES ---
  { id: 'stove-brs', name: 'BRS 3000T Stove', category: 'kitchen', weight: 25, cost: 1500, consumable: false, providedSlots: [] },
  { id: 'pot-toaks', name: 'Toaks 750ml Pot', category: 'kitchen', weight: 100, cost: 3500, consumable: false, providedSlots: [
    { id: 'inside-pot', name: 'Inside Pot', acceptedCategories: ['kitchen', 'fuel', 'consumable'] }
  ]},
  { id: 'fuel-100g', name: 'IsoPro Fuel (100g)', category: 'fuel', weight: 200, cost: 600, consumable: true, providedSlots: [] },
  { id: 'food-day', name: 'Food Bag (1 Day)', category: 'consumable', weight: 700, cost: 1500, consumable: true, providedSlots: [] },
  { id: 'water-1l', name: 'SmartWater 1L (Full)', category: 'consumable', weight: 1000, cost: 200, consumable: true, providedSlots: [] },
  // --- ORGANIZERS ---
  { 
    id: 'ditty-bag', 
    name: 'DCF Ditty Bag', 
    category: 'organizer', 
    weight: 12, 
    cost: 1500, 
    consumable: false,
    providedSlots: [
      { id: 'slot-1', name: 'Item 1', acceptedCategories: ['universal'] },
      { id: 'slot-2', name: 'Item 2', acceptedCategories: ['universal'] },
      { id: 'slot-3', name: 'Item 3', acceptedCategories: ['universal'] },
    ]
  },
  // --- ELECTRONICS/TOOLS ---
  { id: 'bty-nb10000', name: 'Nitecore NB10000', category: 'electronics', weight: 150, cost: 6000, consumable: false, providedSlots: [] },
  { id: 'headlamp-nu25', name: 'Nitecore NU25', category: 'electronics', weight: 28, cost: 3500, consumable: false, providedSlots: [] },
  { id: 'knife-vic', name: 'Victorinox Classic SD', category: 'tool', weight: 21, cost: 2000, consumable: false, providedSlots: [] },
];

const INITIAL_ROOT: LoadoutNode = {
  instanceId: 'root-hiker',
  item: {
    id: 'hiker',
    name: 'My PCT Loadout',
    category: 'system',
    weight: 0,
    cost: 0,
    consumable: false,
    providedSlots: [
      { id: 'worn-torso', name: 'Shirt (Torso)', acceptedCategories: ['shirt'] },
      { id: 'worn-legs', name: 'Pants (Legs)', acceptedCategories: ['pants'] },
      { id: 'worn-feet', name: 'Boots (Feet)', acceptedCategories: ['shoes'] },
      { id: 'carried-pack', name: 'The Backpack', acceptedCategories: ['pack'] },
      { id: 'carried-poles', name: 'Hiking Poles', acceptedCategories: ['poles'] },
    ]
  },
  children: {},
  layout: {
    'worn-torso': 0,
    'worn-legs': 1,
    'worn-feet': 2,
    'carried-poles': 3,
    'carried-pack': 4,
  }
};

/**
 * --- SUB-COMPONENT: LOADOUT EDITOR ---
 * This contains the Stats + Grid logic we built previously.
 */
const LoadoutEditor = () => {
  const [root, setRoot] = useState<LoadoutNode>(INITIAL_ROOT);
  const [path, setPath] = useState<string[]>([]);
  const [pickingFor, setPickingFor] = useState<{ defId: string, nodePath: string[], allowedCats: string[] } | null>(null);
  const [draggedSlotId, setDraggedSlotId] = useState<string | null>(null);

  // --- HELPER LOGIC (Same as before) ---
  const getNodeAtPath = (rootNode: LoadoutNode, pathArr: string[]): LoadoutNode => {
    let current = rootNode;
    for (const step of pathArr) {
      if (current.children[step]) {
        current = current.children[step]!;
      } else {
        return current;
      }
    }
    return current;
  };

  const currentNode = useMemo(() => getNodeAtPath(root, path), [root, path]);

  useEffect(() => {
    const missingLayoutSlots = currentNode.item.providedSlots.filter(
      slot => currentNode.layout[slot.id] === undefined
    );
    if (missingLayoutSlots.length > 0) {
      const newLayout = { ...currentNode.layout };
      const usedIndices = new Set(Object.values(newLayout));
      let nextIndex = 0;
      missingLayoutSlots.forEach(slot => {
        while (usedIndices.has(nextIndex)) nextIndex++;
        newLayout[slot.id] = nextIndex;
        usedIndices.add(nextIndex);
      });
      const updateLayoutInTree = (node: LoadoutNode, targetPath: string[]): LoadoutNode => {
        if (targetPath.length === 0) return { ...node, layout: newLayout };
        const [head, ...tail] = targetPath;
        if (node.children[head]) {
          return {
            ...node,
            children: { ...node.children, [head]: updateLayoutInTree(node.children[head]!, tail) }
          };
        }
        return node;
      };
      setRoot(prev => updateLayoutInTree(prev, path));
    }
  }, [currentNode.item.providedSlots.length, path.length]);

  const stats = useMemo(() => {
    const calc = (node: LoadoutNode): { total: number, base: number, cost: number, count: number } => {
      let t = node.item.weight;
      let b = node.item.consumable ? 0 : node.item.weight;
      let c = node.item.cost;
      let count = 1;
      Object.values(node.children).forEach(child => {
        if (child) {
          const childStats = calc(child);
          t += childStats.total;
          b += childStats.base;
          c += childStats.cost;
          count += childStats.count;
        }
      });
      return { total: t, base: b, cost: c, count };
    };
    return calc(root);
  }, [root]);

  const updateTree = (node: LoadoutNode, pathArr: string[], action: (targetNode: LoadoutNode) => LoadoutNode): LoadoutNode => {
    if (pathArr.length === 0) return action(node);
    const [head, ...tail] = pathArr;
    if (node.children[head]) {
      return {
        ...node,
        children: { ...node.children, [head]: updateTree(node.children[head]!, tail, action) }
      };
    }
    return node;
  };

  const equipItem = (item: Item) => {
    if (!pickingFor) return;
    setRoot(prev => updateTree(prev, pickingFor.nodePath, (node) => {
      const newChildren = { ...node.children };
      newChildren[pickingFor.defId] = {
        instanceId: `${item.id}-${Date.now()}`,
        item: item,
        children: {},
        layout: {} 
      };
      return { ...node, children: newChildren };
    }));
    setPickingFor(null);
  };

  const unequipItem = (slotId: string) => {
    setRoot(prev => updateTree(prev, path, (node) => {
      const newChildren = { ...node.children };
      delete newChildren[slotId];
      return { ...node, children: newChildren };
    }));
  };

  const moveSlot = (slotId: string, newIndex: number) => {
    if (newIndex < 0 || newIndex > 31) return;
    const existingSlotId = Object.keys(currentNode.layout).find(key => currentNode.layout[key] === newIndex);
    setRoot(prev => updateTree(prev, path, (node) => {
      const newLayout = { ...node.layout };
      if (existingSlotId) {
        const oldIndex = newLayout[slotId];
        newLayout[existingSlotId] = oldIndex;
      }
      newLayout[slotId] = newIndex;
      return { ...node, layout: newLayout };
    }));
  };

  const addSlot = () => {
    const name = window.prompt("Enter name for new slot:");
    if (!name) return;
    const newSlotId = `custom-slot-${Date.now()}`;
    const newSlot: SlotDefinition = { id: newSlotId, name: name, acceptedCategories: ['universal'] };
    setRoot(prev => updateTree(prev, path, (node) => {
      const updatedItem = { ...node.item, providedSlots: [...node.item.providedSlots, newSlot] };
      return { ...node, item: updatedItem };
    }));
  };

  const handleDragStart = (e: React.DragEvent, slotId: string) => {
    setDraggedSlotId(slotId);
    e.dataTransfer.effectAllowed = "move";
  };
  const handleDragOver = (e: React.DragEvent) => { e.preventDefault(); e.dataTransfer.dropEffect = "move"; };
  const handleDrop = (e: React.DragEvent, index: number) => {
    e.preventDefault();
    if (draggedSlotId) { moveSlot(draggedSlotId, index); setDraggedSlotId(null); }
  };

  const visibleItems = useMemo(() => {
    if (!pickingFor) return [];
    if (pickingFor.allowedCats.includes('universal')) {
      return MOCK_DB.filter(i => i.category !== 'system' && i.category !== 'pack');
    }
    return MOCK_DB.filter(i => pickingFor.allowedCats.includes(i.category) || pickingFor.allowedCats.includes('universal'));
  }, [pickingFor]);

  const getIcon = (category: string, className?: string) => {
    const cls = (defaultColor: string) => className ? className : `w-5 h-5 ${defaultColor}`;
    switch (category) {
      case 'pack': return <ShoppingBag className={cls("text-blue-400")} />;
      case 'shelter': return <Tent className={cls("text-orange-400")} />;
      case 'consumable': case 'bottle': case 'water': return <Droplets className={cls("text-cyan-400")} />;
      case 'kitchen': return <Utensils className={cls("text-neutral-400")} />;
      case 'electronics': return <Flashlight className={cls("text-yellow-400")} />;
      case 'shirt': return <Shirt className={cls("text-emerald-400")} />;
      case 'pants': return <Layers className={cls("text-emerald-400")} />;
      case 'shoes': return <Footprints className={cls("text-amber-600")} />;
      case 'poles': return <Activity className={cls("text-stone-400")} />;
      case 'universal': return <PackageOpen className={cls("text-stone-500")} />;
      case 'tool': return <Box className={cls("text-red-400")} />;
      default: return <PackageOpen className={cls("text-stone-500")} />;
    }
  };

  const gridCells = useMemo(() => {
    const cells = Array(32).fill(null);
    currentNode.item.providedSlots.forEach(slotDef => {
      const idx = currentNode.layout[slotDef.id];
      if (idx !== undefined && idx < 32) cells[idx] = slotDef;
    });
    return cells;
  }, [currentNode]);

  return (
    <div className="flex h-full w-full">
      {/* Stats Panel */}
      <aside className="w-72 bg-stone-900 border-r border-stone-800 flex flex-col shadow-xl z-20">
        <div className="p-6 border-b border-stone-800 bg-stone-900">
          <h1 className="text-xl font-bold flex items-center gap-2 text-stone-100">
            <Mountain className="w-6 h-6 text-orange-500" />
            BaseWeight
          </h1>
        </div>
        <div className="flex-1 p-6 space-y-6 overflow-y-auto">
          <div className="p-4 rounded-xl bg-orange-500/10 border border-orange-500/20">
            <div className="text-xs font-bold text-orange-400 uppercase tracking-widest mb-1">Base Weight</div>
            <div className="text-3xl font-light text-white">{(stats.base / 1000).toFixed(2)} <span className="text-lg text-stone-500">kg</span></div>
            <div className="text-right text-xs text-orange-300/60 mt-1">Skin-out</div>
          </div>
          <div className="p-4 rounded-xl bg-stone-800 border border-stone-700">
            <div className="text-xs font-bold text-stone-500 uppercase tracking-widest mb-1">Total Weight</div>
            <div className="text-2xl font-light text-white">{(stats.total / 1000).toFixed(2)} <span className="text-sm text-stone-500">kg</span></div>
             <div className="text-xs text-stone-500 mt-2">Consumables: {((stats.total - stats.base) / 1000).toFixed(2)} kg</div>
          </div>
        </div>
      </aside>

      {/* Grid View */}
      <main className="flex-1 flex flex-col relative bg-stone-950 overflow-hidden">
        <header className="h-16 border-b border-stone-800 flex items-center px-8 bg-stone-900/50 backdrop-blur-md sticky top-0 z-10">
           <button onClick={() => setPath([])} className={`text-sm font-medium transition-colors ${path.length === 0 ? 'text-white' : 'text-stone-500 hover:text-white'}`}>Loadout</button>
           {path.map((step, idx) => (
             <React.Fragment key={idx}>
               <ChevronRight className="w-4 h-4 text-stone-600 mx-2" />
               <button onClick={() => setPath(path.slice(0, idx + 1))} className={`text-sm font-medium transition-colors ${idx === path.length - 1 ? 'text-white' : 'text-stone-500 hover:text-white'}`}>{step}</button>
             </React.Fragment>
           ))}
        </header>

        <div className="flex-1 overflow-y-auto p-8">
          <div className="max-w-4xl mx-auto">
            <div className="mb-8 flex justify-between items-end">
              <div>
                <h2 className="text-3xl font-light text-white flex items-center gap-3">{currentNode.item.category !== 'system' && getIcon(currentNode.item.category)}{currentNode.item.name}</h2>
                 <p className="text-stone-500 text-sm mt-1">Drag slots to reorganize. Click slot to equip.</p>
              </div>
              <div className="flex gap-2">
                <button onClick={addSlot} className="flex items-center gap-2 px-4 py-2 bg-stone-800 hover:bg-stone-700 rounded-lg text-sm text-stone-300 transition-colors border border-stone-700"><Plus className="w-4 h-4" /> Add Slot</button>
                {path.length > 0 && <button onClick={() => setPath(path.slice(0, -1))} className="flex items-center gap-2 px-4 py-2 bg-stone-800 hover:bg-stone-700 rounded-lg text-sm text-stone-300 transition-colors border border-stone-700"><ArrowUpLeft className="w-4 h-4" /> Up Level</button>}
              </div>
            </div>

            <div className="grid grid-cols-4 gap-3 select-none" style={{ minHeight: '800px' }}>
              {gridCells.map((slotDef, index) => {
                if (!slotDef) {
                  return (
                    <div key={`empty-${index}`} onDragOver={handleDragOver} onDrop={(e) => handleDrop(e, index)} className="aspect-[4/3] rounded-lg border border-stone-800/50 bg-stone-900/20 hover:bg-stone-800/40 transition-colors flex items-center justify-center">
                      <div className="w-1 h-1 rounded-full bg-stone-800" />
                    </div>
                  );
                }
                const occupied = currentNode.children[slotDef.id];
                const isDragging = draggedSlotId === slotDef.id;
                return (
                  <div key={slotDef.id} draggable onDragStart={(e) => handleDragStart(e, slotDef.id)} onDragOver={handleDragOver} onDrop={(e) => handleDrop(e, index)} className={`relative flex flex-col rounded-xl border transition-all duration-200 overflow-hidden cursor-grab active:cursor-grabbing aspect-[4/3] ${isDragging ? 'opacity-30 scale-95 ring-2 ring-orange-500' : 'opacity-100 scale-100'} ${occupied ? 'bg-stone-900 border-stone-700 shadow-sm' : 'bg-stone-900/40 border-dashed border-stone-800 hover:border-stone-600 hover:bg-stone-900/60'}`}>
                    <div className="flex items-center justify-between p-2 bg-black/20">
                      <div className="flex items-center gap-1.5 overflow-hidden"><GripHorizontal className="w-3 h-3 text-stone-600 shrink-0" /><span className="text-[10px] font-bold text-stone-500 uppercase tracking-widest truncate">{slotDef.name}</span></div>
                      {occupied && (
                        <div className="flex gap-1">
                          <button onClick={() => setPickingFor({ defId: slotDef.id, nodePath: path, allowedCats: slotDef.acceptedCategories })} className="text-stone-600 hover:text-emerald-400" title="Swap Item"><RefreshCw className="w-3 h-3" /></button>
                           {occupied.item.providedSlots.length > 0 && <button onClick={() => setPath([...path, slotDef.id])} className="text-stone-400 hover:text-white" title="Open Container"><Maximize2 className="w-3 h-3" /></button>}
                           <button onClick={() => unequipItem(slotDef.id)} className="text-stone-600 hover:text-red-400" title="Remove Item"><Trash2 className="w-3 h-3" /></button>
                        </div>
                      )}
                    </div>
                    <div className="flex-1 p-2 flex flex-col justify-center">
                      {occupied ? (
                        <>
                          <div className="flex items-center gap-2 mb-1"><div className="text-stone-400">{getIcon(occupied.item.category)}</div><div className="font-medium text-sm text-stone-200 leading-tight line-clamp-2">{occupied.item.name}</div></div>
                          <div className="mt-auto flex justify-between text-[10px] text-stone-500 font-mono"><span>{occupied.item.weight}g</span><span>${(occupied.item.cost / 100).toLocaleString()}</span></div>
                        </>
                      ) : (
                        <button onClick={() => setPickingFor({ defId: slotDef.id, nodePath: path, allowedCats: slotDef.acceptedCategories })} className="flex flex-col items-center justify-center gap-1 h-full text-stone-600 hover:text-orange-400 transition-colors group">
                          <div className="opacity-20 group-hover:opacity-40 transition-opacity">{getIcon(slotDef.acceptedCategories[0] || 'universal', "w-8 h-8 text-stone-500")}</div>
                          <span className="text-[10px] uppercase font-bold text-stone-700 group-hover:text-orange-500/80">Empty</span>
                        </button>
                      )}
                    </div>
                  </div>
                );
              })}
            </div>
          </div>
        </div>

        {/* Modal */}
        {pickingFor && (
          <div className="absolute inset-0 z-50 bg-black/60 backdrop-blur-sm flex items-center justify-center p-4">
            <div className="bg-stone-900 border border-stone-700 w-full max-w-lg rounded-xl shadow-2xl flex flex-col max-h-[85vh] animate-in zoom-in-95 duration-200">
              <div className="p-4 border-b border-stone-800 flex justify-between items-center bg-stone-900">
                <h3 className="font-bold text-white">Select Gear</h3>
                <button onClick={() => setPickingFor(null)} className="p-2 text-stone-500 hover:text-white">✕</button>
              </div>
              <div className="flex-1 overflow-y-auto p-2 bg-stone-950/50">
                <div className="space-y-1">
                  {visibleItems.length > 0 ? (
                    visibleItems.map(item => (
                      <button key={item.id} onClick={() => equipItem(item)} className="w-full text-left p-3 rounded-lg hover:bg-stone-800 border border-transparent hover:border-stone-700 group transition-all flex items-center justify-between">
                        <div className="flex items-center gap-3"><div className="p-2 bg-stone-800 rounded-md text-stone-400 group-hover:text-stone-200">{getIcon(item.category)}</div><div className="text-stone-200 font-medium group-hover:text-orange-400 transition-colors">{item.name}</div></div>
                        <div className="text-right text-stone-300 font-mono text-sm">{item.weight}g</div>
                      </button>
                    ))
                  ) : <div className="py-12 text-center text-stone-500">No gear found.</div>}
                </div>
              </div>
            </div>
          </div>
        )}
      </main>
    </div>
  );
};

// --- PLACEHOLDER VIEWS ---
const HomeView = ({ onOpenLoadout }: { onOpenLoadout: () => void }) => (
  <div className="flex-1 flex flex-col bg-stone-950 text-white p-12">
    <h1 className="text-3xl font-bold mb-8">Welcome Home</h1>
    <div className="grid grid-cols-3 gap-6">
      <div onClick={onOpenLoadout} className="bg-stone-900 border border-stone-800 p-6 rounded-xl hover:border-orange-500 cursor-pointer transition-colors group">
        <div className="w-12 h-12 bg-orange-500/10 rounded-full flex items-center justify-center mb-4 group-hover:bg-orange-500/20 text-orange-500"><Backpack /></div>
        <h3 className="font-bold text-lg mb-1">My PCT Loadout</h3>
        <p className="text-stone-500 text-sm">Last edited 2 mins ago</p>
      </div>
      <div className="bg-stone-900 border border-stone-800 p-6 rounded-xl opacity-50">
        <div className="w-12 h-12 bg-stone-800 rounded-full flex items-center justify-center mb-4 text-stone-600"><Plus /></div>
        <h3 className="font-bold text-lg mb-1">New Loadout</h3>
        <p className="text-stone-500 text-sm">Create a new system</p>
      </div>
    </div>
  </div>
);

const DiscoverView = () => (
  <div className="flex-1 flex flex-col bg-stone-950 text-white p-12 items-center justify-center">
    <Compass className="w-16 h-16 text-stone-800 mb-4" />
    <h2 className="text-xl font-bold text-stone-700">Community Loadouts Coming Soon</h2>
  </div>
);

// --- UPDATED GARAGE VIEW ---
const GarageView = () => {
  const [url, setUrl] = useState('');
  const [isImporting, setIsImporting] = useState(false);
  const [garageItems, setGarageItems] = useState<Item[]>(MOCK_DB); 
  const [newItem, setNewItem] = useState<Partial<Item> | null>(null);

  const handleImport = () => {
    if (!url) return;
    setIsImporting(true);
    
    // Simulate API call and "Link Unfurling"
    setTimeout(() => {
      // Very basic URL parser prototype
      try {
        const urlObj = new URL(url);
        const pathSegments = urlObj.pathname.split('/');
        // Grab the last valid segment for the slug
        const slug = pathSegments.filter(p => p.length > 0).pop() || 'Unknown Item';
        // Clean up slug to make a readable Title Case name
        const itemsName = slug
          .replace(/-/g, ' ')
          .replace(/\.html?$/i, '')
          .replace(/\b\w/g, l => l.toUpperCase());

        setNewItem({
          id: `imported-${Date.now()}`,
          name: itemsName,
          category: 'universal', 
          weight: 0,
          cost: 0,
          consumable: false,
          providedSlots: [],
          description: `Imported from ${urlObj.hostname}`
        });
      } catch (e) {
        alert("Invalid URL");
      }
      setIsImporting(false);
      setUrl('');
    }, 1200);
  };

  const saveItem = () => {
    if (newItem && newItem.name) {
      // Ensure it matches Item interface
      const fullItem: Item = {
        id: newItem.id || `manual-${Date.now()}`,
        name: newItem.name,
        category: newItem.category || 'universal',
        weight: newItem.weight || 0,
        cost: newItem.cost || 0,
        consumable: newItem.consumable || false,
        providedSlots: [],
        description: newItem.description
      };
      setGarageItems(prev => [fullItem, ...prev]); // Add to top
      setNewItem(null);
    }
  };

  return (
    <div className="flex-1 flex flex-col bg-stone-950 text-white p-8 overflow-hidden">
      <div className="flex justify-between items-center mb-8">
        <div>
          <h1 className="text-3xl font-bold mb-2">Gear Garage</h1>
          <p className="text-stone-500">Manage your master inventory database.</p>
        </div>
        <div className="flex gap-2">
             {/* Import Input */}
             <div className="flex bg-stone-900 rounded-lg border border-stone-800 p-1 focus-within:border-orange-500 focus-within:ring-1 focus-within:ring-orange-500 transition-all w-96 shadow-lg">
                <div className="pl-3 flex items-center justify-center text-stone-500"><LinkIcon className="w-4 h-4" /></div>
                <input 
                  type="text" 
                  value={url}
                  onChange={(e) => setUrl(e.target.value)}
                  placeholder="Paste product link (Backcountry, REI...)"
                  className="bg-transparent border-none focus:ring-0 text-sm px-3 flex-1 text-white placeholder:text-stone-600 outline-none"
                  onKeyDown={(e) => e.key === 'Enter' && handleImport()}
                />
                <button 
                  onClick={handleImport}
                  disabled={isImporting || !url}
                  className="bg-stone-800 hover:bg-stone-700 text-stone-200 px-4 py-1.5 rounded-md text-sm font-medium transition-colors disabled:opacity-50 disabled:cursor-not-allowed flex items-center gap-2"
                >
                  {isImporting ? <RefreshCw className="w-4 h-4 animate-spin" /> : "Import"}
                </button>
             </div>
        </div>
      </div>

      {/* Import Review Modal (Inline) */}
      {newItem && (
         <div className="mb-8 bg-stone-900/80 border border-green-500/30 rounded-xl p-6 animate-in slide-in-from-top-4 backdrop-blur-md shadow-2xl">
            <h3 className="font-bold text-lg mb-4 flex items-center gap-2 text-green-400">
              <RefreshCw className="w-5 h-5" />
              Review Imported Gear
            </h3>
            <div className="grid grid-cols-2 gap-4 mb-6">
               <div className="col-span-2">
                 <label className="text-[10px] text-stone-500 uppercase font-bold tracking-wider">Item Name</label>
                 <input type="text" value={newItem.name} onChange={e => setNewItem({...newItem, name: e.target.value})} className="w-full bg-black/40 border border-stone-700 rounded-lg p-2.5 text-white mt-1 focus:border-green-500 focus:outline-none" />
               </div>
               <div>
                 <label className="text-[10px] text-stone-500 uppercase font-bold tracking-wider">Category</label>
                 <select value={newItem.category} onChange={e => setNewItem({...newItem, category: e.target.value})} className="w-full bg-black/40 border border-stone-700 rounded-lg p-2.5 text-white mt-1 focus:border-green-500 focus:outline-none">
                    <option value="pack">Pack</option>
                    <option value="shelter">Shelter</option>
                    <option value="sleep">Sleep</option>
                    <option value="kitchen">Kitchen</option>
                    <option value="clothing">Clothing</option>
                    <option value="electronics">Electronics</option>
                    <option value="universal">Other</option>
                 </select>
               </div>
               <div className="grid grid-cols-2 gap-4">
                 <div>
                   <label className="text-[10px] text-stone-500 uppercase font-bold tracking-wider">Weight (g)</label>
                   <input type="number" value={newItem.weight} onChange={e => setNewItem({...newItem, weight: parseInt(e.target.value) || 0})} className="w-full bg-black/40 border border-stone-700 rounded-lg p-2.5 text-white mt-1 focus:border-green-500 focus:outline-none" />
                 </div>
                 <div>
                   <label className="text-[10px] text-stone-500 uppercase font-bold tracking-wider">Cost (cents)</label>
                   <input type="number" value={newItem.cost} onChange={e => setNewItem({...newItem, cost: parseInt(e.target.value) || 0})} className="w-full bg-black/40 border border-stone-700 rounded-lg p-2.5 text-white mt-1 focus:border-green-500 focus:outline-none" />
                 </div>
               </div>
            </div>
            <div className="flex justify-end gap-3">
              <button onClick={() => setNewItem(null)} className="px-4 py-2 text-stone-400 hover:text-white transition-colors text-sm">Discard</button>
              <button onClick={saveItem} className="px-6 py-2 bg-green-600 hover:bg-green-500 text-white rounded-lg font-medium text-sm transition-colors shadow-lg shadow-green-900/20">Add to Garage</button>
            </div>
         </div>
      )}

      {/* Database Table */}
      <div className="flex-1 overflow-y-auto bg-stone-900/30 rounded-xl border border-stone-800">
         <table className="w-full text-left text-sm">
            <thead className="bg-stone-900 text-stone-500 uppercase font-bold text-[10px] tracking-wider sticky top-0 z-10">
               <tr>
                 <th className="p-4 bg-stone-900">Item Name</th>
                 <th className="p-4 bg-stone-900">Category</th>
                 <th className="p-4 text-right bg-stone-900">Weight</th>
                 <th className="p-4 text-right bg-stone-900">Cost</th>
                 <th className="p-4 bg-stone-900"></th>
               </tr>
            </thead>
            <tbody className="divide-y divide-stone-800/50">
               {garageItems.map(item => (
                 <tr key={item.id} className="hover:bg-stone-800/40 group transition-colors">
                    <td className="p-4 font-medium text-stone-200">{item.name}</td>
                    <td className="p-4 text-stone-500">
                       <span className="px-2 py-1 bg-stone-800 rounded text-xs capitalize text-stone-400">{item.category}</span>
                    </td>
                    <td className="p-4 text-right font-mono text-stone-400">{item.weight}g</td>
                    <td className="p-4 text-right font-mono text-stone-400">${(item.cost / 100).toFixed(2)}</td>
                    <td className="p-4 text-right">
                       <button className="text-stone-600 hover:text-red-400 opacity-0 group-hover:opacity-100 transition-all p-2 hover:bg-stone-800 rounded"><Trash2 className="w-4 h-4" /></button>
                    </td>
                 </tr>
               ))}
            </tbody>
         </table>
      </div>
    </div>
  );
};


export default function App() {
  const [activeTab, setActiveTab] = useState<'home' | 'discover' | 'garage' | 'settings' | 'loadout'>('loadout');

  // Sidebar Tab Button
  const TabButton = ({ id, icon, label, bottom = false }: { id: typeof activeTab, icon: React.ReactNode, label: string, bottom?: boolean }) => {
    const isActive = activeTab === id;
    return (
      <button 
        onClick={() => setActiveTab(id)}
        className={`group relative flex items-center justify-center w-12 h-12 mb-2 transition-all duration-200 ${bottom ? 'mt-auto' : ''}`}
      >
        {/* Active Pill Indicator */}
        <div className={`absolute left-0 w-1 bg-white rounded-r-full transition-all duration-200 ${isActive ? 'h-8' : 'h-0 group-hover:h-4'}`} />
        
        {/* Icon Container */}
        <div className={`
          flex items-center justify-center w-12 h-12 rounded-[24px] transition-all duration-200 overflow-hidden
          ${isActive ? 'bg-orange-500 rounded-[16px] text-white' : 'bg-stone-800 text-stone-400 group-hover:bg-orange-500 group-hover:text-white group-hover:rounded-[16px]'}
        `}>
          {icon}
        </div>
        
        {/* Tooltip (Simple) */}
        <div className="absolute left-16 bg-black text-white text-xs px-2 py-1 rounded opacity-0 group-hover:opacity-100 pointer-events-none whitespace-nowrap z-50 transition-opacity">
          {label}
        </div>
      </button>
    );
  };

  return (
    <div className="flex h-screen bg-stone-950 font-sans overflow-hidden">
      
      {/* 1. DISCORD-STYLE SIDEBAR */}
      <nav className="w-[72px] bg-stone-950 flex flex-col items-center py-3 border-r border-stone-900 z-50">
        <TabButton id="home" icon={<Home size={20} />} label="Home" />
        <TabButton id="discover" icon={<Compass size={20} />} label="Discover" />
        <TabButton id="garage" icon={<Database size={20} />} label="Garage" />
        
        {/* Separator */}
        <div className="w-8 h-[2px] bg-stone-800 rounded-full my-2" />
        
        {/* Specific Loadouts (Servers) */}
        <TabButton id="loadout" icon={<Backpack size={20} />} label="My PCT Loadout" />

        <div className="flex-1" />
        <TabButton id="settings" icon={<Settings size={20} />} label="Settings" />
      </nav>

      {/* 2. MAIN CONTENT AREA */}
      <div className="flex-1 flex overflow-hidden">
        {activeTab === 'home' && <HomeView onOpenLoadout={() => setActiveTab('loadout')} />}
        {activeTab === 'discover' && <DiscoverView />}
        {activeTab === 'garage' && <GarageView />}
        {activeTab === 'loadout' && <LoadoutEditor />}
        {activeTab === 'settings' && <div className="flex-1 bg-stone-950 p-12 text-stone-500">Settings</div>}
      </div>

    </div>
  );
}
